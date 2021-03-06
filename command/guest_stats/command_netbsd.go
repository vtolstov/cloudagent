/*
guest-stats - returns disk and memory stats from guest

Example:
        { "execute": "guest-stats", "arguments": {}}
*/
package guest_stats

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/vtolstov/cloudagent/qga"
)

func init() {
	qga.RegisterCommand(&qga.Command{
		Name:    "guest-stats",
		Func:    fnGuestStats,
		Enabled: true,
		Returns: true,
	})
}

func fnGuestStats(req *qga.Request) *qga.Response {
	res := &qga.Response{ID: req.ID}
	var st unix.Statfs_t

	resData := struct {
		MemoryTotal uint64
		MemoryFree  uint64
		SwapTotal   uint64
		SwapFree    uint64
		BlkTotal    uint64
		BlkFree     uint64
		InodeTotal  uint64
		InodeFree   uint64
	}{}

	buf, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		res.Error = &qga.Error{Code: -1, Desc: err.Error()}
		return res
	}

	reader := bufio.NewReader(bytes.NewBuffer(buf))

	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		fields := strings.Fields(string(line))
		value, err := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(fields[0]) {
		case "MemTotal:":
			resData.MemoryTotal = value * 1024
		case "MemFree:", "Cached:", "Buffers:":
			resData.MemoryFree += value * 1024
		case "SwapTotal:":
			resData.SwapTotal = value * 1024
		case "SwapFree:":
			resData.SwapFree = value * 1024
		}
	}

	err = unix.Stat("/", &st)
	if err != nil {
		res.Error = &qga.Error{Code: -1, Desc: err.Error()}
		return res
	}

	resData.BlkTotal = uint64(st.F_blocks) * uint64(st.F_bsize)
	resData.BlkFree = uint64(st.F_bavail) * uint64(st.F_bsize)

	resData.InodeTotal = uint64(st.F_favail)
	resData.InodeFree = uint64(st.F_ffree)

	res.Return = resData
	return res
}
