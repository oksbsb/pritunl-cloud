package deploy

import (
	"github.com/Sirupsen/logrus"
	"github.com/dropbox/godropbox/container/set"
	"github.com/dropbox/godropbox/errors"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/errortypes"
	"github.com/pritunl/pritunl-cloud/event"
	"github.com/pritunl/pritunl-cloud/instance"
	"github.com/pritunl/pritunl-cloud/node"
	"github.com/pritunl/pritunl-cloud/qemu"
	"github.com/pritunl/pritunl-cloud/qms"
	"github.com/pritunl/pritunl-cloud/state"
	"github.com/pritunl/pritunl-cloud/store"
	"github.com/pritunl/pritunl-cloud/utils"
	"github.com/pritunl/pritunl-cloud/vm"
	"github.com/pritunl/pritunl-cloud/vpc"
	"strings"
	"time"
)

var (
	instancesLock = utils.NewMultiTimeoutLock(3 * time.Minute)
)

type Instances struct {
	stat *state.State
}

func (s *Instances) create(inst *instance.Instance) {
	if instancesLock.Locked(inst.Id.Hex()) {
		return
	}

	lockId := instancesLock.LockTimeout(inst.Id.Hex(), 10*time.Minute)
	go func() {
		defer func() {
			time.Sleep(3 * time.Second)
			instancesLock.Unlock(inst.Id.Hex(), lockId)
		}()

		db := database.GetDatabase()
		defer db.Close()

		err := qemu.Create(db, inst, inst.Virt)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to create instance")
			return
		}

		event.PublishDispatch(db, "instance.change")
	}()
}

func (s *Instances) start(inst *instance.Instance) {
	if instancesLock.Locked(inst.Id.Hex()) {
		return
	}

	lockId := instancesLock.Lock(inst.Id.Hex())
	go func() {
		defer func() {
			time.Sleep(3 * time.Second)
			instancesLock.Unlock(inst.Id.Hex(), lockId)
		}()

		db := database.GetDatabase()
		defer db.Close()

		err := qemu.PowerOn(db, inst, inst.Virt)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to start instance")
			return
		}

		event.PublishDispatch(db, "instance.change")
	}()
}

func (s *Instances) stop(inst *instance.Instance) {
	if instancesLock.Locked(inst.Id.Hex()) {
		return
	}

	lockId := instancesLock.Lock(inst.Id.Hex())
	go func() {
		defer func() {
			time.Sleep(3 * time.Second)
			instancesLock.Unlock(inst.Id.Hex(), lockId)
		}()

		db := database.GetDatabase()
		defer db.Close()

		err := qemu.PowerOff(db, inst.Virt)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to stop instance")
			return
		}

		event.PublishDispatch(db, "instance.change")
	}()
}

func (s *Instances) restart(inst *instance.Instance) {
	if instancesLock.Locked(inst.Id.Hex()) {
		return
	}

	lockId := instancesLock.Lock(inst.Id.Hex())
	go func() {
		defer func() {
			time.Sleep(3 * time.Second)
			instancesLock.Unlock(inst.Id.Hex(), lockId)
		}()

		db := database.GetDatabase()
		defer db.Close()

		err := qemu.PowerOn(db, inst, inst.Virt)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to restart instance")
			return
		}

		time.Sleep(1 * time.Second)

		err = qemu.PowerOff(db, inst.Virt)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to restart instance")
			return
		}

		inst.State = instance.Start
		err = inst.CommitFields(db, set.NewSet("state"))
		if err != nil {
			return
		}

		event.PublishDispatch(db, "instance.change")
	}()
}

func (s *Instances) destroy(inst *instance.Instance) {
	if instancesLock.Locked(inst.Id.Hex()) {
		return
	}

	lockId := instancesLock.Lock(inst.Id.Hex())
	go func() {
		defer func() {
			time.Sleep(3 * time.Second)
			instancesLock.Unlock(inst.Id.Hex(), lockId)
		}()

		db := database.GetDatabase()
		defer db.Close()

		err := qemu.Destroy(db, inst.Virt)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to power off instance")
			return
		}

		err = instance.Remove(db, inst.Id)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to remove instance")
			return
		}

		event.PublishDispatch(db, "instance.change")
		event.PublishDispatch(db, "disk.change")
	}()
}

func (s *Instances) diskRemove(inst *instance.Instance, remDisks []*vm.Disk) {
	if instancesLock.Locked(inst.Id.Hex()) {
		return
	}

	lockId := instancesLock.Lock(inst.Id.Hex())
	go func() {
		defer func() {
			time.Sleep(3 * time.Second)
			instancesLock.Unlock(inst.Id.Hex(), lockId)
		}()

		db := database.GetDatabase()
		defer db.Close()

		for _, dsk := range remDisks {
			e := qms.RemoveDisk(inst.Id, dsk)
			if e != nil {
				logrus.WithFields(logrus.Fields{
					"error": e,
				}).Error("sync: Failed to remove disk")
				return
			}
		}

		event.PublishDispatch(db, "instance.change")
		event.PublishDispatch(db, "disk.change")
	}()
}

func (s *Instances) diff(db *database.Database,
	inst *instance.Instance) (err error) {

	curVirt := s.stat.GetVirt(inst.Id)
	changed := inst.Changed(curVirt)
	addDisks, remDisks := inst.DiskChanged(curVirt)
	if len(addDisks) > 0 {
		changed = true
	}

	if instancesLock.Locked(inst.Id.Hex()) {
		return
	}

	if changed && !inst.Restart {
		inst.Restart = true
		err = inst.CommitFields(db, set.NewSet("restart"))
		if err != nil {
			return
		}
	} else if !changed && inst.Restart {
		inst.Restart = false
		err = inst.CommitFields(db, set.NewSet("restart"))
		if err != nil {
			return
		}
	}

	if len(remDisks) > 0 {
		s.diskRemove(inst, remDisks)
	}

	return
}

func (s *Instances) check(inst *instance.Instance, namespaces set.Set) (
	valid bool, err error) {

	namespace := vm.GetNamespace(inst.Id, 0)
	if !namespaces.Contains(namespace) {
		logrus.WithFields(logrus.Fields{
			"instance_id":   inst.Id.Hex(),
			"net_namespace": namespace,
		}).Error("deploy: Instance missing namespace")
		return
	}

	valid = true

	return
}

func (s *Instances) routes(inst *instance.Instance) (err error) {
	if instancesLock.Locked(inst.Id.Hex()) {
		return
	}

	lockId := instancesLock.Lock(inst.Id.Hex())
	go func() {
		defer func() {
			instancesLock.Unlock(inst.Id.Hex(), lockId)
		}()

		vc := s.stat.Vpc(inst.Vpc)
		if vc == nil {
			err = &errortypes.NotFoundError{
				errors.New("deploy: Instance vpc not found"),
			}
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("deploy: Failed to deploy instance routes")
			return
		}

		namespace := vm.GetNamespace(inst.Id, 0)

		curRoutes := set.NewSet()
		curRoutes6 := set.NewSet()
		newRoutes := set.NewSet()
		newRoutes6 := set.NewSet()

		var routes []vpc.Route
		var routes6 []vpc.Route

		routesStore, ok := store.GetRoutes(inst.Id)
		if !ok {
			routes, routes6, err = qemu.GetRoutes(inst.Id)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"error": err,
				}).Error("deploy: Failed to deploy instance routes")
				return
			}

			if routes == nil || routes6 == nil {
				return
			}

			store.SetRoutes(inst.Id, routes, routes6)
		} else {
			routes = routesStore.Routes
			routes6 = routesStore.Routes6
		}

		for _, route := range routes {
			curRoutes.Add(route)
		}

		for _, route := range routes6 {
			curRoutes6.Add(route)
		}

		if vc.Routes != nil {
			for _, route := range vc.Routes {
				if !strings.Contains(route.Destination, ":") {
					newRoutes.Add(*route)
				} else {
					newRoutes6.Add(*route)
				}
			}
		}

		changed := false
		addRoutes := newRoutes.Copy()
		addRoutes6 := newRoutes6.Copy()
		remRoutes := curRoutes.Copy()
		remRoutes6 := curRoutes6.Copy()

		addRoutes.Subtract(curRoutes)
		addRoutes6.Subtract(curRoutes6)
		remRoutes.Subtract(newRoutes)
		remRoutes6.Subtract(newRoutes6)

		for routeInf := range remRoutes.Iter() {
			route := routeInf.(vpc.Route)
			changed = true

			utils.ExecCombinedOutputLogged(
				nil,
				"ip", "netns", "exec", namespace,
				"ip", "route",
				"del", route.Destination,
				"via", route.Target,
				"metric", "97",
			)
		}

		for routeInf := range remRoutes6.Iter() {
			route := routeInf.(vpc.Route)
			changed = true

			utils.ExecCombinedOutputLogged(
				nil,
				"ip", "netns", "exec", namespace,
				"ip", "-6", "route",
				"del", route.Destination,
				"via", route.Target,
				"metric", "97",
			)
		}

		for routeInf := range addRoutes.Iter() {
			route := routeInf.(vpc.Route)
			changed = true

			utils.ExecCombinedOutputLogged(
				[]string{
					"File exists",
				},
				"ip", "netns", "exec", namespace,
				"ip", "route",
				"add", route.Destination,
				"via", route.Target,
				"metric", "97",
			)
		}

		for routeInf := range addRoutes6.Iter() {
			route := routeInf.(vpc.Route)
			changed = true

			utils.ExecCombinedOutputLogged(
				[]string{
					"File exists",
				},
				"ip", "netns", "exec", namespace,
				"ip", "-6", "route",
				"add", route.Destination,
				"via", route.Target,
				"metric", "97",
			)
		}

		if changed {
			store.RemRoutes(inst.Id)
		}
	}()

	return
}

func (s *Instances) Deploy() (err error) {
	db := database.GetDatabase()
	defer db.Close()

	instances := s.stat.Instances()
	namespaces := s.stat.Namespaces()

	namespacesSet := set.NewSet()
	for _, namespace := range namespaces {
		namespacesSet.Add(namespace)
	}

	cpuUnits := 0
	memoryUnits := 0.0

	for _, inst := range instances {
		curVirt := s.stat.GetVirt(inst.Id)

		if inst.State == instance.Destroy {
			s.destroy(inst)
			continue
		}

		cpuUnits += inst.Processors
		memoryUnits += float64(inst.Memory) / float64(1024)

		if curVirt == nil {
			s.create(inst)
			continue
		}

		switch inst.State {
		case instance.Start:
			if curVirt.State == vm.Stopped || curVirt.State == vm.Failed {
				s.start(inst)
				continue
			}

			valid, e := s.check(inst, namespacesSet)
			if e != nil {
				err = e
				return
			}
			if !valid {
				continue
			}

			err = s.diff(db, inst)
			if err != nil {
				return
			}

			err = s.routes(inst)
			if err != nil {
				return
			}

			break
		case instance.Stop:
			if curVirt.State == vm.Running {
				s.stop(inst)
				continue
			}
			break
		case instance.Restart:
			if curVirt.State == vm.Running {
				s.restart(inst)
				continue
			}
			break
		}
	}

	node.Self.CpuUnitsRes = cpuUnits
	node.Self.MemoryUnitsRes = memoryUnits

	return
}

func NewInstances(stat *state.State) *Instances {
	return &Instances{
		stat: stat,
	}
}
