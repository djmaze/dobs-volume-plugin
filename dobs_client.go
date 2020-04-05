
package main

import (
	"log"
	"fmt"
	"time"
	"strconv"
	"errors"
	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

// DobsClient ...
type DobsClient struct {
	GodoClient *godo.Client
	Region string
}

// APIVolume ...
type APIVolume struct {
	Name string
	ID string
	// DropletID int
}

// Client returns a new DigitalOcean client
func Client(token string, region string) (DobsClient, error) {
	tokenSrc := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})

	client, err := godo.New(oauth2.NewClient(
		oauth2.NoContext, tokenSrc),
		godo.SetUserAgent(userAgent()))

	return DobsClient{GodoClient: client, Region: region}, err
}

// GetVolume returns information about a volume from the DO API
func (d DobsClient) GetVolume(ctx Context, name string) (*APIVolume, error) {

	apiVolume, err := d.getVolumeByName(ctx, name)
	if err != nil {
		return nil, err
	}

	vol := &APIVolume{
		Name: apiVolume.Name,
		ID: apiVolume.ID,
		// DropletID: apiVolume.DropletIDs[0],
	}

	return vol, nil
}

// AttachVolume attaches the given volume to the given droplet
func (d DobsClient) AttachVolume(ctx Context, volumeID string, dropletID string) (error) {
	dropletIDI, err := strconv.Atoi(dropletID)
	if err != nil {
		return err
	}

	vol, _, err := d.GodoClient.Storage.GetVolume(ctx, volumeID)
	if err != nil {
		return err
	}

	if len(vol.DropletIDs) > 0 {
		otherDropletID := vol.DropletIDs[0]
		if otherDropletID == dropletIDI {
			log.Printf("Volume %s already attached to this droplet, skipping attach\n", volumeID)
			return nil
		}
		
		return fmt.Errorf("Volume %s already attached to different droplet %d", volumeID, otherDropletID)
	}

	action, _, err := d.GodoClient.StorageActions.Attach(ctx, volumeID, dropletIDI)	
	if err != nil {
		return err
	}

	err = d.waitForAction(ctx, volumeID, action)
	if err != nil {
		return err
	}

	return nil
}

// DetachVolume detaches the given volume from the given droplet
func (d DobsClient) DetachVolume(ctx Context, volumeID string, dropletID string) (error) {
	dropletIDI, err := strconv.Atoi(dropletID)
	if err != nil {
		return err
	}

	action, _, err := d.GodoClient.StorageActions.DetachByDropletID(ctx, volumeID, dropletIDI)	
	if err != nil {
		return err
	}

	err = d.waitForAction(ctx, volumeID, action)
	if err != nil {
		return err
	}

	return nil
}


func (d DobsClient) waitForAction(ctx Context, volumeID string, action *godo.Action) error {

	// TODO Cleanup
    time.Sleep(time.Duration(10) * time.Second)
	f := func() (error) {
		time.Sleep(time.Duration(2) * time.Second)
		parsedDuration, _ := time.ParseDuration("100ms")
		duration := parsedDuration.Nanoseconds()
		maxAttempts := 5
		for i := 1; i <= maxAttempts; i++ {
			action, _, err := d.GodoClient.StorageActions.Get(
				ctx, volumeID, action.ID)
			if err != nil {
				return err
			}
			if action.Status == godo.ActionCompleted {
				return nil
			}
			log.Println(
				"still waiting for action",
			)
			time.Sleep(time.Duration(duration) * time.Nanosecond)
			duration = int64(2) * duration
		}
		return errors.New("Status attempts exhausted")
	}

	statusTimeout, _ := time.ParseDuration("1m")
	ok, err := WaitFor(f, statusTimeout)
	if !ok {
		return errors.New("Timeout occured waiting for storage action")
	}
	if err != nil {
		return err
	}
	return nil
}

func (d DobsClient) getVolumeByName(ctx Context, name string) (*godo.Volume, error) {
	listOpts := &godo.ListVolumeParams{
		// ListOptions: &godo.ListOptions{PerPage: 200},
		Region:      d.Region,
		Name: name,
	}

	doVolumes, _, err := d.GodoClient.Storage.ListVolumes(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	if len(doVolumes) == 0 {
		return nil, errors.New("Could not find volume by name")
	}
	if len(doVolumes) > 1 {
		return nil, errors.New("too many volumes returned")
	}

	return &doVolumes[0], nil
}

func userAgent() string {
	return "dobs-volume-driver/0.1"
}

// WaitFor waits for a lambda to complete or aborts after a specified amount
// of time. If the function fails to complete in the specified amount of time
// then the second return value is a boolean false. Otherwise the 
// possible error of the provided lambda is returned.
func WaitFor(
	f func() (error),
	timeout time.Duration) (bool, error) {

	var (
		err error

		fc = make(chan bool, 1)
	)

	go func() {
		err = f()
		fc <- true
	}()
	tc := time.After(timeout)

	select {
	case <-fc:
		return true, err
	case <-tc:
		return false, nil
	}
}