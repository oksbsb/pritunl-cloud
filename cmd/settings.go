package cmd

import (
	"encoding/json"
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/dropbox/godropbox/errors"
	"github.com/pritunl/pritunl-cloud/config"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/errortypes"
	"github.com/pritunl/pritunl-cloud/settings"
	"github.com/pritunl/pritunl-cloud/user"
	"gopkg.in/mgo.v2/bson"
)

func Mongo() (err error) {
	mongodbUri := flag.Arg(1)

	err = config.Load()
	if err != nil {
		return
	}

	config.Config.MongoUri = mongodbUri

	err = config.Save()
	if err != nil {
		return
	}

	logrus.WithFields(logrus.Fields{
		"mongo_uri": config.Config.MongoUri,
	}).Info("cmd: Set MongoDB URI")

	return
}

func ResetId() (err error) {
	err = config.Load()
	if err != nil {
		return
	}

	config.Config.NodeId = bson.NewObjectId().Hex()

	err = config.Save()
	if err != nil {
		return
	}

	logrus.WithFields(logrus.Fields{
		"node_id": config.Config.NodeId,
	}).Info("cmd: Reset node ID")

	return
}

func ResetPassword() (err error) {
	db := database.GetDatabase()
	defer db.Close()

	coll := db.Users()

	err = coll.Remove(&bson.M{
		"username": "pritunl",
	})
	if err != nil {
		if _, ok := err.(*database.NotFoundError); ok {
			err = nil
		} else {
			return
		}
	}

	usr := user.User{
		Type:          user.Local,
		Username:      "pritunl",
		Administrator: "super",
	}

	err = usr.SetPassword("pritunl")
	if err != nil {
		return
	}

	err = usr.Insert(db)
	if err != nil {
		return
	}

	logrus.WithFields(logrus.Fields{
		"username": "pritunl",
		"password": "pritunl",
	}).Info("cmd: Password reset")

	return
}

func SettingsSet() (err error) {
	group := flag.Arg(1)
	key := flag.Arg(2)
	val := flag.Arg(3)
	db := database.GetDatabase()
	defer db.Close()

	var valParsed interface{}
	err = json.Unmarshal([]byte(val), &valParsed)
	if err != nil {
		err = &errortypes.ParseError{
			errors.Wrap(err, "cmd.settings: Failed to parse value"),
		}
		return
	}

	err = settings.Set(db, group, key, valParsed)
	if err != nil {
		return
	}

	return
}

func SettingsUnset() (err error) {
	group := flag.Arg(1)
	key := flag.Arg(2)
	db := database.GetDatabase()
	defer db.Close()

	err = settings.Unset(db, group, key)
	if err != nil {
		return
	}

	return
}
