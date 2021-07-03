package obfs

type Base struct {
	Host   string
	Port   int
	Key    []byte
	IVSize int
	Param  string
}
