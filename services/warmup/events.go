package warmup

const (
	TopicName        = "warmup"
	wwrmupKickedName = TopicName + ".kicked"
)

type WarmupKicked struct {
	UID string
}

func (e WarmupKicked) GetEventTypeName() string {
	return wwrmupKickedName
}

func (e WarmupKicked) GetAggregateName() string {
	return e.UID
}
