package vo

var (
	Pending = newStatus(0, "Pending")
	Success = newStatus(1, "Success")
	Failed  = newStatus(2, "Failed")
)

type Status struct {
	statusCode byte
	statusMsg  string
}

func newStatus(code byte, msg string) *Status {
	return &Status{
		statusCode: code,
		statusMsg:  msg,
	}
}

func GetStatusByString(s string) *Status {
	switch s {
	case Pending.statusMsg:
		return Pending
	case Success.statusMsg:
		return Success
	case Failed.statusMsg:
		return Failed
	default:
		return nil
	}
}

func GetStatusByCode(code byte) *Status {
	switch code {
	case Pending.statusCode:
		return Pending
	case Success.statusCode:
		return Success
	case Failed.statusCode:
		return Failed
	default:
		return nil
	}
}

func (s *Status) GetCode() byte {
	return s.statusCode
}

func (s *Status) GetMsg() string {
	return s.statusMsg
}
