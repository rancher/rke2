package shared

// A LocalGroup represents a locally defined group on a Windows system.
type LocalGroup struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

// A LocalGroupMember contains information about a member of a group.
type LocalGroupMember struct {
	Domain        string `json:"domain"`
	Name          string `json:"name"`
	DomainAndName string `json:"domainAndName"`
}
