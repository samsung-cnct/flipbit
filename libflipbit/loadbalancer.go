package libflipbit

type LoadBalancer struct {
	URL string
	Timeout int64
}

type LoadBalancers []LoadBalancer