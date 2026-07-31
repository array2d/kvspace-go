package kvspace

// ── Dict ─────────────────────────────────────────────────────────────────

type Dict struct{}

func (Dict) Kind() string    { return KindDict }
func (Dict) ByteLen() int32  { return 0 }
func (Dict) ArrayLen() int32 { return 1 }
func (Dict) Encode() []byte  { return TLVEncode(KindDict, nil, 1) }
