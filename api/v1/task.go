package v1

type TaskSubmitRequest struct {
	Language string `json:"language,required" vd:"len($)>0"`
	Code     string `json:"code,required" vd:"len($)>0"`
}

type TaskSubmitResponseBody struct {
	TaskID string `json:"task_id"`
}

type TaskSubmitResponse struct {
	Response
	TaskSubmitResponseBody `json:"data"`
}

type TaskResultResponseBody struct {
	TaskID   string `json:"task_id"`
	Language string `json:"language"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

type TaskResultResponse struct {
	Response
	TaskResultResponseBody `json:"data"`
}
