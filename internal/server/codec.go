package server

import (
	"encoding/json"

	"github.com/go-kratos/kratos/v3/encoding"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// init overrides Kratos's default "json" codec so that proto Timestamps are
// serialized as RFC 3339 strings instead of {"seconds":X,"nanos":Y} objects.
// UseProtoNames keeps field names in snake_case to match the existing frontend.
func init() {
	encoding.RegisterCodec(protoJSONCodec{})
}

type protoJSONCodec struct{}

var (
	protoMarshal = protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: false,
	}
	protoUnmarshal = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

func (protoJSONCodec) Marshal(v any) ([]byte, error) {
	if m, ok := v.(proto.Message); ok {
		return protoMarshal.Marshal(m)
	}
	return json.Marshal(v)
}

func (protoJSONCodec) Unmarshal(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	if m, ok := v.(proto.Message); ok {
		return protoUnmarshal.Unmarshal(data, m)
	}
	return json.Unmarshal(data, v)
}

func (protoJSONCodec) Name() string {
	return "json"
}
