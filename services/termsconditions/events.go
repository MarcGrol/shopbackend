package termsconditions

const (
	TopicName    = "termsconditions"
	acceptedName = TopicName + ".accepted"
)

type TermsConditionsAccepted struct {
	EmailAddress string
	Version      string
}

func (e TermsConditionsAccepted) GetEventTypeName() string {
	return acceptedName
}

func (e TermsConditionsAccepted) GetAggregateName() string {
	return e.EmailAddress
}
