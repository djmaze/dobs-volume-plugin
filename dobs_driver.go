package main

import (
	"syscall"
	"os"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"log"
	"time"
	volumeplugin "github.com/docker/go-plugins-helpers/volume"
)

// Context is our context
type Context interface {
	context.Context
	// WithValue(key, value interface{}) Context
}

type dobsVolume struct {
	VolumeID string
	Name string
	Mountpoint string
	connections int
	shouldDetach bool
}

type dobsDriver struct {
	sync.RWMutex

	root string
  baseURL string
	token string
	InstanceID string
	Region string
	client         DobsClient
	volumes map[string]*dobsVolume
}

func newDobsDriver(root string, token string, baseURL string) (*dobsDriver) {
	d := &dobsDriver{
    root:      filepath.Join(root, "volumes"),
	  volumes:   map[string]*dobsVolume{},
    baseURL: baseURL,
	  token: token,
	}

	return d
}

func (d *dobsDriver) Init() error {
	ctx := context.Background()

	isDroplet, err := IsDroplet(ctx)
	if err != nil {
		return err
	}

	if !isDroplet {
		return errors.New("Host is not a droplet")
	}

	droplet, err := Instance(ctx)
	if err != nil {
		return err
	}
	d.InstanceID = droplet.ID
	d.Region = droplet.Region

	client, err := Client(
		d.token,
		d.Region,
    d.baseURL,
	)
	if err != nil {
		return err
	}
	d.client = client

	// fields := map[string]interface{}{
	// }

	log.Printf("Storage driver initialized. Droplet ID %s, Region %s, root %s\n",
		d.InstanceID,
		d.Region,
		d.root,
	)

	return nil
}

func (d *dobsDriver) Capabilities() *volumeplugin.CapabilitiesResponse {
  return &volumeplugin.CapabilitiesResponse{Capabilities: volumeplugin.Capability{Scope: "local"}}
}

func (d *dobsDriver) Create(req *volumeplugin.CreateRequest) error {
  d.Lock()
  defer d.Unlock()

	ctx := context.Background()

	volName := req.Name
	for key, val := range req.Options {
		switch key {
		case "name":
			volName = val
		default:
			return fmt.Errorf("unknown option %q", val)
		}
	}

	apiVolume, err := d.client.GetVolume(ctx, volName)
	if err != nil {
		return err
	}

	v := &dobsVolume{
		VolumeID: apiVolume.ID,
		Name: volName,	
	}
	v.Mountpoint = filepath.Join(d.root, req.Name)
	d.volumes[req.Name] = v

	return nil
}

func (d *dobsDriver) Get(req *volumeplugin.GetRequest) (*volumeplugin.GetResponse, error) {
	var res volumeplugin.GetResponse

  d.RLock()
  defer d.RUnlock()

  v, ok := d.volumes[req.Name]
  if !ok {
    return nil, errors.New(req.Name)
  }
	res.Volume = &volumeplugin.Volume{
		Name:       req.Name,
		Mountpoint: filepath.Join(v.Mountpoint, "data"),
	}
	return &res, nil
}

func (d *dobsDriver) List() (*volumeplugin.ListResponse, error) {
  d.RLock()
  defer d.RUnlock()

  var vols []*volumeplugin.Volume
  for name, v := range d.volumes {
    vols = append(vols, &volumeplugin.Volume{
		Name: name, 
		Mountpoint: filepath.Join(v.Mountpoint, "data"),
	})
  }
  return &volumeplugin.ListResponse{Volumes: vols}, nil
}

func (d *dobsDriver) Mount(req *volumeplugin.MountRequest) (*volumeplugin.MountResponse, error) {
	var res volumeplugin.MountResponse

	d.Lock()
	defer d.Unlock()

	ctx := context.Background()

	v, ok := d.volumes[req.Name]
	if !ok {
		return nil, errors.New(req.Name)
	}

	v.shouldDetach = false

	if v.connections == 0 {
		err := d.attachVolume(ctx, v)
		if err != nil {
			return nil, err
		}

		fi, err := os.Lstat(v.Mountpoint)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(v.Mountpoint, 0755); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}

		if fi != nil && !fi.IsDir() {
			return nil, fmt.Errorf("%v already exist and it's not a directory", v.Mountpoint)
		}

		if err := d.mountVolume(v); err != nil {
			return nil, err
		}
    }
	v.connections++

	res.Mountpoint = filepath.Join(v.Mountpoint, "data")
	return &res, nil
}

func (d *dobsDriver) mountVolume(v *dobsVolume) (error) {
	src := fmt.Sprintf("/dev/disk/by-id/scsi-0DO_Volume_%s", v.Name)

	log.Printf("Mounting %s: device %s at %s\n",
		v.Name,
		src,
		v.Mountpoint,
	)

	err := syscall.Mount(src, v.Mountpoint, "ext4", 0, "")
	if err != nil {
		log.Printf("Error mounting %s: %v", v.Name, err)
		return err
	}

	return nil
}

func (d *dobsDriver) Path(req *volumeplugin.PathRequest) (*volumeplugin.PathResponse, error) {
	var res volumeplugin.PathResponse

  d.RLock()
  defer d.RUnlock()

  v, ok := d.volumes[req.Name]
  if !ok {
    return nil, errors.New(req.Name)
  }

	res.Mountpoint = filepath.Join(v.Mountpoint, "data")
	log.Printf("Returning path %s for volume %s", res.Mountpoint, v.Name)
	return &res, nil
}

func (d *dobsDriver) Remove(req *volumeplugin.RemoveRequest) error {
  d.Lock()
  defer d.Unlock()

  v, ok := d.volumes[req.Name]
  if !ok {
    return errors.New(req.Name)
  }

  if v.connections != 0 {
    return fmt.Errorf("Volume still has %d connections", v.connections)
  }

  if err := os.RemoveAll(v.Mountpoint); err != nil {
    return err
  }
  delete(d.volumes, req.Name)
  return nil
}

func (d *dobsDriver) Unmount(req *volumeplugin.UnmountRequest) error {
	d.Lock()
	defer d.Unlock()

	v, ok := d.volumes[req.Name]
	if !ok {
		// TODO
		return errors.New(req.Name)
	}

	v.connections--

	if v.connections <= 0 {
		if err := d.unmountVolume(v); err != nil {
			return err
		}

		ctx := context.Background()

		if err := d.detachVolume(ctx, v); err != nil {
			return err
		}
	}

	return nil
}

func (d *dobsDriver) unmountVolume(v *dobsVolume) error {
	log.Printf("Unmounting %s from %s\n",
		v.Name,
		v.Mountpoint,
	)

	err := syscall.Unmount(v.Mountpoint, 0)
	if err != nil {
		return err
	}

	return nil
}

func (d *dobsDriver) attachVolume(ctx Context, v *dobsVolume) error {
	log.Printf("Attaching volume %s to droplet %s\n",
		v.Name,
		d.InstanceID,
	)

	return d.client.AttachVolume(ctx, v.VolumeID, d.InstanceID)
}

func (d *dobsDriver) detachVolume(ctx Context, v *dobsVolume) error {
	v.shouldDetach = true

	go d.detachLater(v, 2 * time.Second)
	return nil
}

func (d *dobsDriver) detachLater(v *dobsVolume, n time.Duration) {
	time.Sleep(n)

	d.Lock()
	defer d.Unlock()

	if v.shouldDetach {
		log.Printf("Detaching %s from droplet %s\n",
			v.Name,
			d.InstanceID,
		)
		d.client.DetachVolume(context.Background(), v.VolumeID, d.InstanceID)

		v.shouldDetach = false
	}
}
