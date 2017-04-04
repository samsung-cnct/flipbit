package libflipbit

type Port struct {
	NativePort int32
	NodePort int32
	Protocol string
}

type Ports []Port
