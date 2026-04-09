package adapters

type Adapter interface {
	Name() string
	Init() error
	Destroy() error
}
