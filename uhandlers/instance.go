package uhandlers

import (
	"fmt"
	"github.com/dropbox/godropbox/container/set"
	"github.com/gin-gonic/gin"
	"github.com/pritunl/pritunl-cloud/aggregate"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/datacenter"
	"github.com/pritunl/pritunl-cloud/demo"
	"github.com/pritunl/pritunl-cloud/domain"
	"github.com/pritunl/pritunl-cloud/event"
	"github.com/pritunl/pritunl-cloud/image"
	"github.com/pritunl/pritunl-cloud/instance"
	"github.com/pritunl/pritunl-cloud/node"
	"github.com/pritunl/pritunl-cloud/utils"
	"github.com/pritunl/pritunl-cloud/vm"
	"github.com/pritunl/pritunl-cloud/vpc"
	"github.com/pritunl/pritunl-cloud/zone"
	"gopkg.in/mgo.v2/bson"
	"strconv"
	"strings"
)

type instanceData struct {
	Id           bson.ObjectId `json:"id"`
	Zone         bson.ObjectId `json:"zone"`
	Vpc          bson.ObjectId `json:"vpc"`
	Node         bson.ObjectId `json:"node"`
	Image        bson.ObjectId `json:"image"`
	Domain       bson.ObjectId `json:"domain"`
	Name         string        `json:"name"`
	State        string        `json:"state"`
	InitDiskSize int           `json:"init_disk_size"`
	Memory       int           `json:"memory"`
	Processors   int           `json:"processors"`
	NetworkRoles []string      `json:"network_roles"`
	Count        int           `json:"count"`
}

type instanceMultiData struct {
	Ids   []bson.ObjectId `json:"ids"`
	State string          `json:"state"`
}

type instancesData struct {
	Instances []*aggregate.InstanceAggregate `json:"instances"`
	Count     int                            `json:"count"`
}

func instancePut(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)
	data := &instanceData{}

	instanceId, ok := utils.ParseObjectId(c.Param("instance_id"))
	if !ok {
		utils.AbortWithStatus(c, 400)
		return
	}

	err := c.Bind(data)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	inst, err := instance.GetOrg(db, userOrg, instanceId)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	exists, err := vpc.ExistsOrg(db, userOrg, data.Vpc)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}
	if !exists {
		utils.AbortWithStatus(c, 405)
		return
	}

	if data.Domain != "" {
		exists, err := domain.ExistsOrg(db, userOrg, data.Domain)
		if err != nil {
			utils.AbortWithError(c, 500, err)
			return
		}
		if !exists {
			utils.AbortWithStatus(c, 405)
			return
		}
	}

	inst.PreCommit()

	inst.Name = data.Name
	inst.Vpc = data.Vpc
	if data.State != "" {
		inst.State = data.State
	}
	inst.Memory = data.Memory
	inst.Processors = data.Processors
	inst.NetworkRoles = data.NetworkRoles
	inst.Domain = data.Domain

	fields := set.NewSet(
		"name",
		"vpc",
		"state",
		"restart",
		"memory",
		"processors",
		"network_roles",
		"domain",
	)

	errData, err := inst.Validate(db)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	if errData != nil {
		c.JSON(400, errData)
		return
	}

	err = inst.PostCommit(db)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	err = inst.CommitFields(db, fields)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "instance.change")

	c.JSON(200, inst)
}

func instancePost(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)
	data := &instanceData{
		Name: "New Instance",
	}

	err := c.Bind(data)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	zne, err := zone.Get(db, data.Zone)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	exists, err := datacenter.ExistsOrg(db, userOrg, zne.Datacenter)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}
	if !exists {
		utils.AbortWithStatus(c, 405)
		return
	}

	nde, err := node.Get(db, data.Node)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	if nde.Zone != zne.Id {
		utils.AbortWithStatus(c, 405)
		return
	}

	exists, err = vpc.ExistsOrg(db, userOrg, data.Vpc)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}
	if !exists {
		utils.AbortWithStatus(c, 405)
		return
	}

	if data.Domain != "" {
		exists, err := domain.ExistsOrg(db, userOrg, data.Domain)
		if err != nil {
			utils.AbortWithError(c, 500, err)
			return
		}
		if !exists {
			utils.AbortWithStatus(c, 405)
			return
		}
	}

	exists, err = image.ExistsOrg(db, userOrg, data.Image)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}
	if !exists {
		utils.AbortWithStatus(c, 405)
		return
	}

	insts := []*instance.Instance{}

	if data.Count == 0 {
		data.Count = 1
	}

	for i := 0; i < data.Count; i++ {
		name := ""
		if strings.Contains(data.Name, "%") {
			name = fmt.Sprintf(data.Name, i+1)
		} else {
			name = data.Name
		}

		inst := &instance.Instance{
			State:        data.State,
			Organization: userOrg,
			Zone:         data.Zone,
			Vpc:          data.Vpc,
			Node:         data.Node,
			Image:        data.Image,
			Name:         name,
			InitDiskSize: data.InitDiskSize,
			Memory:       data.Memory,
			Processors:   data.Processors,
			NetworkRoles: data.NetworkRoles,
			Domain:       data.Domain,
		}

		errData, err := inst.Validate(db)
		if err != nil {
			utils.AbortWithError(c, 500, err)
			return
		}

		if errData != nil {
			c.JSON(400, errData)
			return
		}

		err = inst.Insert(db)
		if err != nil {
			utils.AbortWithError(c, 500, err)
			return
		}

		insts = append(insts, inst)
	}

	event.PublishDispatch(db, "instance.change")

	if len(insts) == 1 {
		c.JSON(200, insts[0])
	} else {
		c.JSON(200, insts)
	}
}

func instancesPut(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)
	data := &instanceMultiData{}

	err := c.Bind(data)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	doc := bson.M{
		"state": data.State,
	}

	if data.State != instance.Start {
		doc["restart"] = false
	}

	err = instance.UpdateMultiOrg(db, userOrg, data.Ids, &doc)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "instance.change")

	c.JSON(200, nil)
}

func instanceDelete(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)

	instanceId, ok := utils.ParseObjectId(c.Param("instance_id"))
	if !ok {
		utils.AbortWithStatus(c, 400)
		return
	}

	err := instance.DeleteOrg(db, userOrg, instanceId)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "instance.change")

	c.JSON(200, nil)
}

func instancesDelete(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)
	data := []bson.ObjectId{}

	err := c.Bind(&data)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	force := c.Query("force")
	if force == "true" {
		for _, instId := range data {
			err = instance.Remove(db, instId)
			if err != nil {
				utils.AbortWithError(c, 500, err)
				return
			}
		}
	} else {
		err = instance.DeleteMultiOrg(db, userOrg, data)
		if err != nil {
			utils.AbortWithError(c, 500, err)
			return
		}
	}

	event.PublishDispatch(db, "instance.change")

	c.JSON(200, nil)
}

func instanceGet(c *gin.Context) {
	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)

	instanceId, ok := utils.ParseObjectId(c.Param("instance_id"))
	if !ok {
		utils.AbortWithStatus(c, 400)
		return
	}

	inst, err := instance.GetOrg(db, userOrg, instanceId)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	if demo.IsDemo() {
		inst.State = instance.Start
		inst.VmState = vm.Running
		inst.Status = "Running"
		inst.PublicIps = []string{
			demo.RandIp(inst.Id),
		}
		inst.PublicIps6 = []string{
			demo.RandIp6(inst.Id),
		}
	}

	c.JSON(200, inst)
}

func instancesGet(c *gin.Context) {
	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)

	nde, _ := utils.ParseObjectId(c.Query("node"))
	if nde != "" {
		query := &bson.M{
			"node":         nde,
			"organization": userOrg,
		}

		insts, err := instance.GetAllName(db, query)
		if err != nil {
			utils.AbortWithError(c, 500, err)
			return
		}

		c.JSON(200, insts)
	} else {
		page, _ := strconv.Atoi(c.Query("page"))
		pageCount, _ := strconv.Atoi(c.Query("page_count"))

		query := bson.M{
			"organization": userOrg,
		}

		instId, ok := utils.ParseObjectId(c.Query("id"))
		if ok {
			query["_id"] = instId
		}

		name := strings.TrimSpace(c.Query("name"))
		if name != "" {
			query["name"] = &bson.M{
				"$regex":   fmt.Sprintf(".*%s.*", name),
				"$options": "i",
			}
		}

		networkRole := strings.TrimSpace(c.Query("network_role"))
		if networkRole != "" {
			query["network_roles"] = networkRole
		}

		instances, count, err := aggregate.GetInstancePaged(
			db, &query, page, pageCount)
		if err != nil {
			utils.AbortWithError(c, 500, err)
			return
		}

		for _, inst := range instances {
			inst.Json()

			if demo.IsDemo() {
				inst.State = instance.Start
				inst.VmState = vm.Running
				inst.Status = "Running"
				inst.PublicIps = []string{
					demo.RandIp(inst.Id),
				}
				inst.PublicIps6 = []string{
					demo.RandIp6(inst.Id),
				}
			}
		}

		data := &instancesData{
			Instances: instances,
			Count:     count,
		}

		c.JSON(200, data)
	}
}
