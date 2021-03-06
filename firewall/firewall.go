package firewall

import (
	"github.com/dropbox/godropbox/container/set"
	"github.com/dropbox/godropbox/errors"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/errortypes"
	"gopkg.in/mgo.v2/bson"
	"net"
	"strconv"
	"strings"
)

type Rule struct {
	SourceIps []string `bson:"source_ips" json:"source_ips"`
	Protocol  string   `bson:"protocol" json:"protocol"`
	Port      string   `bson:"port" json:"port"`
}

type Firewall struct {
	Id           bson.ObjectId `bson:"_id,omitempty" json:"id"`
	Name         string        `bson:"name" json:"name"`
	Organization bson.ObjectId `bson:"organization,omitempty" json:"organization"`
	NetworkRoles []string      `bson:"network_roles" json:"network_roles"`
	Ingress      []*Rule       `bson:"ingress" json:"ingress"`
}

func (f *Firewall) Validate(db *database.Database) (
	errData *errortypes.ErrorData, err error) {

	if f.NetworkRoles == nil {
		f.NetworkRoles = []string{}
	}

	if f.Ingress == nil {
		f.Ingress = []*Rule{}
	}

	for _, rule := range f.Ingress {
		switch rule.Protocol {
		case All:
			rule.Port = ""
			break
		case Icmp:
			rule.Port = ""
			break
		case Tcp, Udp:
			ports := strings.Split(rule.Port, "-")

			portInt, e := strconv.Atoi(ports[0])
			if e != nil {
				errData = &errortypes.ErrorData{
					Error:   "invalid_ingress_rule_port",
					Message: "Invalid ingress rule port",
				}
				return
			}

			if portInt < 1 || portInt > 65535 {
				errData = &errortypes.ErrorData{
					Error:   "invalid_ingress_rule_port",
					Message: "Invalid ingress rule port",
				}
				return
			}

			parsedPort := strconv.Itoa(portInt)
			if len(ports) > 1 {
				portInt2, e := strconv.Atoi(ports[1])
				if e != nil {
					errData = &errortypes.ErrorData{
						Error:   "invalid_ingress_rule_port",
						Message: "Invalid ingress rule port",
					}
					return
				}

				if portInt < 1 || portInt > 65535 || portInt2 <= portInt {
					errData = &errortypes.ErrorData{
						Error:   "invalid_ingress_rule_port",
						Message: "Invalid ingress rule port",
					}
					return
				}

				parsedPort += "-" + strconv.Itoa(portInt2)
			}

			rule.Port = parsedPort

			break
		default:
			errData = &errortypes.ErrorData{
				Error:   "invalid_ingress_rule_protocol",
				Message: "Invalid ingress rule protocol",
			}
			return
		}

		for i, sourceIp := range rule.SourceIps {
			if sourceIp == "" {
				errData = &errortypes.ErrorData{
					Error:   "invalid_ingress_rule_source_ip",
					Message: "Empty ingress rule source IP",
				}
				return
			}

			if !strings.Contains(sourceIp, "/") {
				if strings.Contains(sourceIp, ":") {
					sourceIp += "/128"
				} else {
					sourceIp += "/32"
				}
			}

			_, sourceCidr, e := net.ParseCIDR(sourceIp)
			if e != nil {
				errData = &errortypes.ErrorData{
					Error:   "invalid_ingress_rule_source_ip",
					Message: "Invalid ingress rule source IP",
				}
				return
			}

			rule.SourceIps[i] = sourceCidr.String()
		}
	}

	return
}

func (f *Firewall) Commit(db *database.Database) (err error) {
	coll := db.Firewalls()

	err = coll.Commit(f.Id, f)
	if err != nil {
		return
	}

	return
}

func (f *Firewall) CommitFields(db *database.Database, fields set.Set) (
	err error) {

	coll := db.Firewalls()

	err = coll.CommitFields(f.Id, f, fields)
	if err != nil {
		return
	}

	return
}

func (f *Firewall) Insert(db *database.Database) (err error) {
	coll := db.Firewalls()

	if f.Id != "" {
		err = &errortypes.DatabaseError{
			errors.New("firewall: Firewall already exists"),
		}
		return
	}

	err = coll.Insert(f)
	if err != nil {
		err = database.ParseError(err)
		return
	}

	return
}
