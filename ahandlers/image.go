package ahandlers

import (
	"fmt"
	"github.com/dropbox/godropbox/container/set"
	"github.com/gin-gonic/gin"
	"github.com/pritunl/pritunl-cloud/data"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/datacenter"
	"github.com/pritunl/pritunl-cloud/demo"
	"github.com/pritunl/pritunl-cloud/event"
	"github.com/pritunl/pritunl-cloud/image"
	"github.com/pritunl/pritunl-cloud/utils"
	"gopkg.in/mgo.v2/bson"
	"strconv"
	"strings"
)

type imageData struct {
	Id           bson.ObjectId `json:"id"`
	Name         string        `json:"name"`
	Organization bson.ObjectId `json:"organization"`
}

type imagesData struct {
	Images []*image.Image `json:"images"`
	Count  int            `json:"count"`
}

func imagePut(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	dta := &imageData{}

	imageId, ok := utils.ParseObjectId(c.Param("image_id"))
	if !ok {
		utils.AbortWithStatus(c, 400)
		return
	}

	err := c.Bind(dta)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	img, err := image.Get(db, imageId)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	img.Name = dta.Name
	img.Organization = dta.Organization

	fields := set.NewSet(
		"name",
		"organization",
	)

	errData, err := img.Validate(db)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	if errData != nil {
		c.JSON(400, errData)
		return
	}

	err = img.CommitFields(db, fields)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "image.change")

	c.JSON(200, img)
}

func imageDelete(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)

	imageId, ok := utils.ParseObjectId(c.Param("image_id"))
	if !ok {
		utils.AbortWithStatus(c, 400)
		return
	}

	err := data.DeleteImage(db, imageId)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "image.change")

	c.JSON(200, nil)
}

func imagesDelete(c *gin.Context) {
	if demo.Blocked(c) {
		return
	}

	db := c.MustGet("db").(*database.Database)
	dta := []bson.ObjectId{}

	err := c.Bind(&dta)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	err = data.DeleteImages(db, dta)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	event.PublishDispatch(db, "image.change")

	c.JSON(200, nil)
}

func imageGet(c *gin.Context) {
	db := c.MustGet("db").(*database.Database)

	imageId, ok := utils.ParseObjectId(c.Param("image_id"))
	if !ok {
		utils.AbortWithStatus(c, 400)
		return
	}

	img, err := image.Get(db, imageId)
	if err != nil {
		utils.AbortWithError(c, 500, err)
		return
	}

	img.Json()

	c.JSON(200, img)
}

func imagesGet(c *gin.Context) {
	db := c.MustGet("db").(*database.Database)

	dcId, _ := utils.ParseObjectId(c.Query("datacenter"))
	if dcId != "" {
		dc, err := datacenter.Get(db, dcId)
		if err != nil {
			return
		}

		storages := dc.PublicStorages
		if storages == nil {
			storages = []bson.ObjectId{}
		}

		if len(storages) == 0 {
			c.JSON(200, []bson.ObjectId{})
			return
		}

		query := &bson.M{
			"storage": &bson.M{
				"$in": storages,
			},
		}

		images, err := image.GetAllNames(db, query)
		if err != nil {
			utils.AbortWithError(c, 500, err)
			return
		}

		for _, img := range images {
			img.Json()
		}

		if dc.PrivateStorage != "" {
			query = &bson.M{
				"storage": dc.PrivateStorage,
			}

			images2, err := image.GetAllNames(db, query)
			if err != nil {
				utils.AbortWithError(c, 500, err)
				return
			}

			for _, img := range images2 {
				img.Json()
				images = append(images, img)
			}
		}

		c.JSON(200, images)
	} else {
		page, _ := strconv.Atoi(c.Query("page"))
		pageCount, _ := strconv.Atoi(c.Query("page_count"))

		query := bson.M{}

		imageId, ok := utils.ParseObjectId(c.Query("id"))
		if ok {
			query["_id"] = imageId
		}

		name := strings.TrimSpace(c.Query("name"))
		if name != "" {
			query["$or"] = []*bson.M{
				&bson.M{
					"name": &bson.M{
						"$regex":   fmt.Sprintf(".*%s.*", name),
						"$options": "i",
					},
				},
				&bson.M{
					"key": &bson.M{
						"$regex":   fmt.Sprintf(".*%s.*", name),
						"$options": "i",
					},
				},
			}
		}

		typ := strings.TrimSpace(c.Query("type"))
		if typ != "" {
			query["type"] = typ
		}

		organization, ok := utils.ParseObjectId(c.Query("organization"))
		if ok {
			query["organization"] = organization
		}

		images, count, err := image.GetAll(db, &query, page, pageCount)
		if err != nil {
			utils.AbortWithError(c, 500, err)
			return
		}

		for _, img := range images {
			img.Json()
		}

		dta := &imagesData{
			Images: images,
			Count:  count,
		}

		c.JSON(200, dta)
	}
}
