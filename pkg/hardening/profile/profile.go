package profile

const (
	// Valid hardening profile names
	ProfileNameNone = ""
	ProfileNameCIS  = "cis"
	ProfileNameETCD = "etcd"

	ModeNone Mode = iota
	ModeCIS
	ModeETCD
)

type Mode int

func (m Mode) IsAnyMode() bool {
	return m != ModeNone
}

func (m Mode) IsCISMode() bool {
	return m == ModeCIS
}

func FromString(profileName string) Mode {
	switch profileName {
	case ProfileNameCIS:
		return ModeCIS
	case ProfileNameETCD:
		return ModeETCD
	default:
		return ModeNone
	}
}
