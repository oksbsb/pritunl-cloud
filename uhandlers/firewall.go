package uhandlers

import (
	"fmt"
	"github.com/dropbox/godropbox/container/set"
	"github.com/gin-gonic/gin"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/demo"
	"github.com/pritunl/pritunl-cloud/event"
	"github.com/pritunl/pritunl-cloud/firewall"
	"github.com/pritunl/pritunl-cloud/utils"
	"gopkg.in/mgo.v2/bson"
	"strconv"
	"strings"
)

type firewallData struct {
	Id           bson.ObjectId    `json:"id"`
	Name         string           `json:"name"`
	NetworkRoles []string         `json:"network_roles"`
	Ingress      []*firewall.Rule `json:"ingress"`
}

type firewallsData struct {
	Firewalls []*firewall.Firewall `json:"firewalls"`
	Count     int                  `json:"count"`
}

func firewallPut(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)
	data := &firewallData{}

	firewallId, ok := utils.ParseObjectId(c.Param("firewall_id"))
	if !ok {
		utils.AbortWithStatus(c, 400)
		return
	}

	err := c.Bind(data)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	fire, err := firewall.GetOrg(db, userOrg, firewallId)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	fire.Name = data.Name
	fire.NetworkRoles = data.NetworkRoles
	fire.Ingress = data.Ingress

	fields := set.NewSet(
		"state",
		"name",
		"network_roles",
		"ingress",
	)

	errData, err := fire.Validate(db)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	if errData != nil {
		c.JSON(400, errData)
		return
	}

	err = fire.CommitFields(db, fields)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "firewall.change")

	c.JSON(200, fire)
}

func firewallPost(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)
	data := &firewallData{
		Name: "New Firewall",
	}

	err := c.Bind(data)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	fire := &firewall.Firewall{
		Name:         data.Name,
		Organization: userOrg,
		NetworkRoles: data.NetworkRoles,
		Ingress:      data.Ingress,
	}

	errData, err := fire.Validate(db)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	if errData != nil {
		c.JSON(400, errData)
		return
	}

	err = fire.Insert(db)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "firewall.change")

	c.JSON(200, fire)
}

func firewallDelete(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)

	firewallId, ok := utils.ParseObjectId(c.Param("firewall_id"))
	if !ok {
		utils.AbortWithStatus(c, 400)
		return
	}

	err := firewall.RemoveOrg(db, userOrg, firewallId)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "firewall.change")

	c.JSON(200, nil)
}

func firewallsDelete(c *gin.Context) {
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

	err = firewall.RemoveMultiOrg(db, userOrg, data)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "firewall.change")

	c.JSON(200, nil)
}

func firewallGet(c *gin.Context) {
	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)

	firewallId, ok := utils.ParseObjectId(c.Param("firewall_id"))
	if !ok {
		utils.AbortWithStatus(c, 400)
		return
	}

	fire, err := firewall.GetOrg(db, userOrg, firewallId)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	c.JSON(200, fire)
}

func firewallsGet(c *gin.Context) {
	db := c.MustGet("db").(*database.Database)
	userOrg := c.MustGet("organization").(bson.ObjectId)

	page, _ := strconv.Atoi(c.Query("page"))
	pageCount, _ := strconv.Atoi(c.Query("page_count"))

	query := bson.M{
		"organization": userOrg,
	}

	firewallId, ok := utils.ParseObjectId(c.Query("id"))
	if ok {
		query["_id"] = firewallId
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

	firewalls, count, err := firewall.GetAllPaged(db, &query, page, pageCount)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	data := &firewallsData{
		Firewalls: firewalls,
		Count:     count,
	}

	c.JSON(200, data)
}
