package qemu

import (
	"fmt"
	"github.com/pritunl/pritunl-cloud/utils"
	"github.com/pritunl/pritunl-cloud/vm"
	"github.com/pritunl/pritunl-cloud/vpc"
	"gopkg.in/mgo.v2/bson"
	"net"
	"strings"
)

func GetRoutes(instId bson.ObjectId) (routes []vpc.Route,
	routes6 []vpc.Route, err error) {

	namespace := vm.GetNamespace(instId, 0)

	output, _ := utils.ExecCombinedOutputLogged(
		[]string{
			"not configured in this system",
		},
		"ip", "netns", "exec", namespace,
		"route", "-n",
	)
	//if err != nil {
	//	return
	//}

	if output == "" {
		return
	}

	routes = []vpc.Route{}
	routes6 = []vpc.Route{}

	lines := strings.Split(output, "\n")
	if len(lines) > 2 {
		for _, line := range lines[2:] {
			if line == "" {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) < 8 {
				continue
			}

			if fields[4] != "97" {
				continue
			}

			if fields[0] == "0.0.0.0" || fields[1] == "0.0.0.0" {
				continue
			}

			mask := utils.ParseIpMask(fields[2])
			if mask == nil {
				continue
			}
			cidr, _ := mask.Size()

			route := vpc.Route{
				Destination: fmt.Sprintf("%s/%d", fields[0], cidr),
				Target:      fields[1],
			}

			routes = append(routes, route)
		}
	}

	output, _ = utils.ExecCombinedOutputLogged(
		[]string{
			"not configured in this system",
		},
		"ip", "netns", "exec", namespace,
		"route", "-6", "-n",
	)
	//if err != nil {
	//	return
	//}

	lines = strings.Split(output, "\n")
	if len(lines) > 2 {
		for _, line := range lines[2:] {
			if line == "" {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) < 7 {
				continue
			}

			if fields[3] != "97" {
				continue
			}

			_, destination, e := net.ParseCIDR(fields[0])
			if e != nil || destination == nil {
				continue
			}

			target := net.ParseIP(fields[1])
			if target == nil {
				continue
			}

			route := vpc.Route{
				Destination: destination.String(),
				Target:      target.String(),
			}

			routes6 = append(routes6, route)
		}
	}

	return
}
