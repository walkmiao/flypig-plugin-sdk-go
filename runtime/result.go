package runtime

import pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"

func Success() *pluginapi.OperationResult {
	return &pluginapi.OperationResult{Ok: true}
}

func Failure(code pluginapi.ErrorCode, message string, retryable bool) *pluginapi.OperationResult {
	return &pluginapi.OperationResult{
		Ok: false,
		Error: &pluginapi.ErrorDetail{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
	}
}

func Unsupported(capability string) *pluginapi.OperationResult {
	return Failure(
		pluginapi.ERROR_CODE_UNSUPPORTED_CAPABILITY,
		"plugin capability is not implemented: "+capability,
		false,
	)
}
