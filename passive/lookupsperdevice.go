package passive

import (
	"regexp"

	"code.google.com/p/goprotobuf/proto"
	"github.com/sburnett/transformer"
	"github.com/sburnett/transformer/key"
	"github.com/sburnett/transformer/store"
)

func LookupsPerDevicePipeline(tracesStore, availabilityIntervalsStore, addressIdStore store.Seeker, addressIdToDomainStore store.SeekingWriter, lookupsPerDeviceSharded store.ReadingWriter, lookupsPerDeviceStore, lookupsPerDevicePerHourStore store.Writer, workers int) []transformer.PipelineStage {
	consistentTracesStore := store.NewRangeIncludingReader(tracesStore, availabilityIntervalsStore)
	return []transformer.PipelineStage{
		transformer.PipelineStage{
			Name:        "LookupsPerDeviceMapper",
			Reader:      consistentTracesStore,
			Transformer: transformer.MakeDoFunc(lookupsPerDeviceMapper, workers),
			Writer:      addressIdToDomainStore,
		},
		transformer.PipelineStage{
			Name:        "JoinMacWithLookups",
			Reader:      store.NewDemuxingSeeker(addressIdStore, addressIdToDomainStore),
			Transformer: transformer.TransformFunc(joinMacWithLookups),
			Writer:      lookupsPerDeviceSharded,
		},
		transformer.PipelineStage{
			Name:        "FlattenLookupsToNodeAndMac",
			Reader:      lookupsPerDeviceSharded,
			Transformer: transformer.TransformFunc(flattenLookupsToNodeAndMac),
			Writer:      lookupsPerDeviceStore,
		},
		transformer.PipelineStage{
			Name:        "FlattenLookupsToNodeMacAndTimestamp",
			Reader:      lookupsPerDeviceSharded,
			Transformer: transformer.TransformFunc(flattenLookupsToNodeMacAndTimestamp),
			Writer:      lookupsPerDevicePerHourStore,
		},
	}
}

func lookupsPerDeviceMapper(record *store.Record, outputChan chan *store.Record) {
	mobileDomainRegexp, err := regexp.Compile(`(^m\.|\.m\.)`)
	if err != nil {
		panic(err)
	}

	var traceKey TraceKey
	key.DecodeOrDie(record.Key, &traceKey)
	var trace Trace
	if err := proto.Unmarshal(record.Value, &trace); err != nil {
		panic(err)
	}

	allDomains := make(map[int32]map[string]int64)
	for _, entry := range trace.ARecord {
		if entry.AddressId == nil || entry.Domain == nil || entry.Anonymized == nil {
			continue
		}
		if *entry.Anonymized {
			continue
		}
		if !mobileDomainRegexp.MatchString(*entry.Domain) {
			continue
		}

		if _, ok := allDomains[*entry.AddressId]; !ok {
			allDomains[*entry.AddressId] = make(map[string]int64)
		}
		allDomains[*entry.AddressId][*entry.Domain]++
	}
	for _, entry := range trace.CnameRecord {
		if entry.AddressId == nil || entry.Domain == nil || entry.DomainAnonymized == nil || entry.Cname == nil || entry.CnameAnonymized == nil {
			continue
		}
		if _, ok := allDomains[*entry.AddressId]; !ok {
			allDomains[*entry.AddressId] = make(map[string]int64)
		}
		if !*entry.DomainAnonymized && mobileDomainRegexp.MatchString(*entry.Domain) {
			allDomains[*entry.AddressId][*entry.Domain]++
		}
		if !*entry.CnameAnonymized && mobileDomainRegexp.MatchString(*entry.Cname) {
			allDomains[*entry.AddressId][*entry.Cname]++
		}
	}

	for addressId, domainsMap := range allDomains {
		for domain, count := range domainsMap {
			outputChan <- &store.Record{
				Key:   key.EncodeOrDie(traceKey.SessionKey(), addressId, traceKey.SequenceNumber, domain),
				Value: key.EncodeOrDie(count),
			}
		}
	}
}

func joinMacWithLookups(inputChan, outputChan chan *store.Record) {
	var (
		session   SessionKey
		addressId int32
	)
	grouper := transformer.GroupRecords(inputChan, &session, &addressId)
	for grouper.NextGroup() {
		var macAddress []byte
		for grouper.NextRecord() {
			record := grouper.Read()
			switch record.DatabaseIndex {
			case 0:
				key.DecodeOrDie(record.Value, &macAddress)
			case 1:
				if macAddress != nil {
					var (
						sequenceNumber int32
						domain         string
					)
					key.DecodeOrDie(record.Key, &sequenceNumber, &domain)
					outputChan <- &store.Record{
						Key:   key.EncodeOrDie(session.NodeId, macAddress, domain, session.AnonymizationContext, session.SessionId, sequenceNumber),
						Value: record.Value,
					}
				}
			}
		}
	}
}

func flattenLookupsToNodeAndMac(inputChan, outputChan chan *store.Record) {
	var nodeId, macAddress, domain string
	grouper := transformer.GroupRecords(inputChan, &nodeId, &macAddress, &domain)
	for grouper.NextGroup() {
		var totalCount int64
		for grouper.NextRecord() {
			record := grouper.Read()
			var count int64
			key.DecodeOrDie(record.Value, &count)
			totalCount += count
		}
		outputChan <- &store.Record{
			Key:   key.EncodeOrDie(nodeId, macAddress, domain),
			Value: key.EncodeOrDie(totalCount),
		}
	}
}

func flattenLookupsToNodeMacAndTimestamp(inputChan, outputChan chan *store.Record) {
	var nodeId, macAddress, domain string
	grouper := transformer.GroupRecords(inputChan, &nodeId, &macAddress, &domain)
	for grouper.NextGroup() {
		totalCounts := make(map[int64]int64)
		for grouper.NextRecord() {
			record := grouper.Read()
			var (
				anonymizationContext string
				sessionId            int64
				sequenceNumber       int32
			)
			key.DecodeOrDie(record.Key, &anonymizationContext, &sessionId, &sequenceNumber)
			var count int64
			key.DecodeOrDie(record.Value, &count)
			timestamp := truncateTimestampToHour(sessionId + convertSecondsToMicroseconds(30)*int64(sequenceNumber))
			totalCounts[timestamp] += count
		}
		for timestamp, totalCount := range totalCounts {
			outputChan <- &store.Record{
				Key:   key.EncodeOrDie(nodeId, macAddress, domain, timestamp),
				Value: key.EncodeOrDie(totalCount),
			}
		}
	}
}