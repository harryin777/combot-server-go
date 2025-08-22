package task

import (
	"combot-server-go/src/core/utils"
	"context"
)

type CallBack struct {
	taskCallback func(result interface{})
}

func NewCallBack(callback func(result interface{})) *CallBack {
	return &CallBack{
		taskCallback: callback,
	}
}

func (cb *CallBack) OnComplete(result interface{}) {
	if cb.taskCallback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					utils.WithField(context.Background(), "panic", r).Error("Callback panic recovered")
				}
			}()
			cb.taskCallback(result)
		}()
	}
}

func (cb *CallBack) OnError(err error) {
	if cb.taskCallback != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					utils.WithField(context.Background(), "panic", r).Error("Error callback panic recovered")
				}
			}()
			result := map[string]interface{}{
				"error":  err.Error(),
				"status": "failed",
			}
			cb.taskCallback(result)
		}()
	}
}
