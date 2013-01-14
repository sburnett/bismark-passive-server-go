package passive

import (
	"archive/tar"
	"code.google.com/p/goprotobuf/proto"
	"compress/gzip"
	"expvar"
	"fmt"
	"github.com/sburnett/transformer"
	"github.com/sburnett/transformer/key"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var tarBytesRead, tarsFailed, tracesFailed *expvar.Int

func init() {
	tarBytesRead = expvar.NewInt("TarBytesRead")
	tarsFailed = expvar.NewInt("TarsFailed")
	tracesFailed = expvar.NewInt("TracesFailed")
}

func traceKey(trace *Trace) []byte {
	var anonymizationContext string
	if trace.AnonymizationSignature != nil {
		anonymizationContext = *trace.AnonymizationSignature
	}
	return key.EncodeOrDie(
		*trace.NodeId,
		anonymizationContext,
		*trace.ProcessStartTimeMicroseconds,
		*trace.SequenceNumber)
}

func indexTarball(tarPath string, tracesChan chan *transformer.LevelDbRecord) bool {
	handle, err := os.Open(tarPath)
	if err != nil {
		log.Printf("Error reading %s: %s\n", tarPath, err)
		tarsFailed.Add(int64(1))
		return false
	}
	defer handle.Close()
	fileinfo, err := handle.Stat()
	if err != nil {
		log.Printf("Error stating %s: %s\n", tarPath, err)
		tarsFailed.Add(int64(1))
		return false
	}
	tarBytesRead.Add(fileinfo.Size())
	tr := tar.NewReader(handle)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			tarsFailed.Add(1)
			log.Printf("Error indexing %v: %v", tarPath, err)
			break
		}
		unzippedReader, err := gzip.NewReader(tr)
		if err != nil {
			tracesFailed.Add(1)
			log.Printf("%s/%s: %v", tarPath, header.Name, err)
			continue
		}
		contents, err := ioutil.ReadAll(unzippedReader)
		if err != nil {
			tracesFailed.Add(1)
			log.Printf("%s/%s: %v", tarPath, header.Name, err)
			continue
		}

		trace, err := parseTrace(contents)
		if err != nil {
			tracesFailed.Add(1)
			continue
		}
		key := traceKey(trace)
		value, err := proto.Marshal(trace)
		if err != nil {
			panic(fmt.Errorf("Error encoding protocol buffer: %v", err))
		}
		tracesChan <- &transformer.LevelDbRecord{
			Key:   key,
			Value: value,
		}
	}
	return true
}

func IndexTarballs(inputRecords []*transformer.LevelDbRecord, outputChan ...chan *transformer.LevelDbRecord) {
	if len(inputRecords) != 1 || inputRecords[0].DatabaseIndex != 0 {
		return
	}

	tracesChan := outputChan[0]
	tarnamesChan := outputChan[1]

	var tarPath string
	key.DecodeOrDie(inputRecords[0].Key, &tarPath)
	if indexTarball(tarPath, tracesChan) {
		tarnamesChan <- &transformer.LevelDbRecord{
			Key: key.EncodeOrDie(tarPath),
		}
	}
}
