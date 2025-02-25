package events

type LogWriter struct {
	eventMgr *Manager
}

func (w LogWriter) Write(inputBytes []byte) (n int, err error) {
	msg := string(inputBytes)

	w.eventMgr.SendEvent(Log{Data: msg})

	return len(inputBytes), nil
}
