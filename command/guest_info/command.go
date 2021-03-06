/*
Package guest_info - request agent info from guest

Example:
        { "execute": "guest-info", "arguments": {}}
*/
package guest_info

import (
	"github.com/vtolstov/cloudagent/qga"
)

func init() {
	qga.RegisterCommand(&qga.Command{
		Name:    "guest-info",
		Func:    fnGuestInfo,
		Enabled: true,
		Returns: true,
	})
}

func fnGuestInfo(req *qga.Request) *qga.Response {
	res := &qga.Response{ID: req.ID}

	res.Return = struct {
		Version  string         `json:"version"`
		Uptime   int64          `json:"uptime"`
		Commands []*qga.Command `json:"supported_commands"`
	}{Version: qga.GetVersion(), Commands: qga.ListCommands(), Uptime: int64(qga.GetUptime().Unix())}

	return res
}
