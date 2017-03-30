package libflipbit

type LBHost struct {
	Chance int
	Host string
}

type LBHosts []LBHost


func (slice LBHosts) Len() int {
	return len(slice)
}

func (slice LBHosts) Less(i, j int) bool {
	return slice[i].Chance < slice[j].Chance
}

func (slice LBHosts) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}
