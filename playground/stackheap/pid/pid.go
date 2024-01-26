package pid

import "fmt"

type Name string
type LocalPID string
type NetworkPID string

type PID struct {
	Name
	LocalPID
	NetworkPID
	Parent *PID
}

func NewPID(parent *PID, name string) *PID {
	return &PID{
		Name:     Name(name),
		Parent:   parent,
		LocalPID: getLocalPID(parent, name),
	}
}

func (p *PID) String() string {
	return fmt.Sprintf("%v@%v", p.LocalPID, p.NetworkPID)
}

func getLocalPID(parent *PID, name string) LocalPID {
	var reclocal func(p *PID) LocalPID
	reclocal = func(p *PID) LocalPID {
		return LocalPID(fmt.Sprintf("%v/%v", p.Name, reclocal(p)))
	}
	return LocalPID(fmt.Sprintf("%v/%v", reclocal(parent), name))
}
