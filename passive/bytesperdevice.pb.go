// Code generated by protoc-gen-go.
// source: bytesperdevice.proto
// DO NOT EDIT!

package passive

import proto "code.google.com/p/goprotobuf/proto"
import json "encoding/json"
import math "math"

// Reference proto, json, and math imports to suppress error if they are not otherwise used.
var _ = proto.Marshal
var _ = &json.SyntaxError{}
var _ = math.Inf

type BytesPerTimestampEntry struct {
	Timestamp        *int64 `protobuf:"varint,1,opt,name=timestamp" json:"timestamp,omitempty"`
	Size             *int64 `protobuf:"varint,2,opt,name=size" json:"size,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (this *BytesPerTimestampEntry) Reset()         { *this = BytesPerTimestampEntry{} }
func (this *BytesPerTimestampEntry) String() string { return proto.CompactTextString(this) }
func (*BytesPerTimestampEntry) ProtoMessage()       {}

func (this *BytesPerTimestampEntry) GetTimestamp() int64 {
	if this != nil && this.Timestamp != nil {
		return *this.Timestamp
	}
	return 0
}

func (this *BytesPerTimestampEntry) GetSize() int64 {
	if this != nil && this.Size != nil {
		return *this.Size
	}
	return 0
}

type BytesPerTimestamp struct {
	Entry            []*BytesPerTimestampEntry `protobuf:"bytes,1,rep,name=entry" json:"entry,omitempty"`
	XXX_unrecognized []byte                    `json:"-"`
}

func (this *BytesPerTimestamp) Reset()         { *this = BytesPerTimestamp{} }
func (this *BytesPerTimestamp) String() string { return proto.CompactTextString(this) }
func (*BytesPerTimestamp) ProtoMessage()       {}

func init() {
}
