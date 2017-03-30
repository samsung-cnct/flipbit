package libflipbit

type Entry struct {
	Name string
	Namespace string
	NodePorts []int32
	Hosts []string
	LoadBalancers []string
	Remained bool
	Changed bool
}

type Entries []Entry
