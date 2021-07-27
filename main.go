package main

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	currentSessionCount = -1
	needsReconnect      = false
	needsReconnectTime  = time.Time{}
)

func checkReconnect(timeout int, ifname string, smsClient *SMSClient) {
	if !needsReconnect {
		return
	}

	now := time.Now()

	if !now.After(needsReconnectTime.Add(time.Duration(timeout) * time.Second)) {
		//d := math.Round(float64(ReconnectIntervalSeconds - (now.Sub(needsReconnectTime) / time.Second)))
		//log.Infof("Redialing in %v seconds", d)
		return
	}

	log.Info("Redial connection")
	redial(ifname, smsClient)
}

func ipAddr(ifname string) (net.Addr, error) {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, errors.New("no address assigned")
	}

	return addrs[0], nil
}

func redial(ifname string, sms *SMSClient) {
	addr, err := ipAddr(ifname)
	if err != nil {
		log.Errorf("Error getting current IP: %s", err)
		return
	}

	hangup := exec.Command("/usr/bin/killall", "-HUP", "pppd")
	if err := hangup.Run(); err != nil {
		log.Error("Error sending signal to pppd")
		return
	}
	needsReconnect = false

	go func() {
		for {
			newAddr, err := ipAddr(ifname)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			if sms != nil {
				go func() {
					sms.Send(fmt.Sprintf("VPN IP updated. \nNew IP: %s \nOld IP: %s", newAddr.String(), addr.String()))
				}()
			}

			log.
				WithFields(map[string]interface{}{"oldIP": addr.String(), "newIP": newAddr.String()}).
				Infof("Redialed")
			break
		}
	}()
}

func init() {
	log.SetFormatter(&log.TextFormatter{TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true})
}

func main() {
	endpoint := ""
	pflag.StringVarP(&endpoint, "endpoint", "s", "https://localhost:5555/api", "api endpoint for vpnserver")

	ifname := ""
	pflag.StringVarP(&ifname, "interface", "i", "ppp0", "wan interface name for checking ip")

	redialTimeout := 0
	pflag.IntVarP(&redialTimeout, "redial-timeout", "t", 900, "number of seconds to wait until redial when server has no active sessions")

	smsUsername := ""
	pflag.StringVar(&smsUsername, "sms-username", "", "username for sms service from every8d")

	smsPassword := ""
	pflag.StringVar(&smsPassword, "sms-password", "", "password for sms service from every8d")

	var smsNotifyNumbers []string
	pflag.StringArrayVar(&smsNotifyNumbers, "sms-notify-number", []string{}, "phone number to notify using sms for updates, specify multiple times to set multiple numbers")

	pflag.Parse()

	var smsClient *SMSClient
	if smsUsername != "" && smsPassword != "" && len(smsNotifyNumbers) > 0 {
		smsClient = NewSMSClient(smsUsername, smsPassword, smsNotifyNumbers)
	} else {
		smsClient = nil
	}

	c := NewSoftEtherAPIClient(endpoint)

	loc, err := time.LoadLocation("Local")
	if err != nil {
		log.Fatal(err)
	}

	sched := gocron.NewScheduler(loc)

	sched.Every("1s").SingletonMode().Do(func() {
		ss, err := c.EnumSession("DEFAULT")
		if err != nil {
			log.Errorf("Get sessions error: %s", err)
			return
		}
		sc := len(ss)

		if currentSessionCount != sc {
			log.Infof("Current VPN sessions: %v", sc)

			if sc == 0 {
				needsReconnect = true
				needsReconnectTime = time.Now()
			} else {
				needsReconnect = false
			}
		}

		if sc == 0 {
			checkReconnect(redialTimeout, ifname, smsClient)
		}

		currentSessionCount = sc
	})

	log.Info("Starting vpnmon")
	sched.StartBlocking()
}
