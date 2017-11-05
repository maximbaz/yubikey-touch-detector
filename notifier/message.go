package notifier

type Message string

const (
	GPG_ON  Message = "GPG_ON"
	GPG_OFF Message = "GPG_OFF"
	U2F_ON  Message = "U2F_ON"
	U2F_OFF Message = "U2F_OFF"
)
