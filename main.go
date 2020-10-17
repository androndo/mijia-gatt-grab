package main

import (
	//"flag"
	"fmt"
	"strconv"

	//"os"
	"strings"
	"time"

	"github.com/paypal/gatt"
	"github.com/paypal/gatt/examples/option"
	log "github.com/sirupsen/logrus"
)

var done = make(chan string)

func onStateChanged(d gatt.Device, s gatt.State) {
	fmt.Println("State:", s)
	switch s {
	case gatt.StatePoweredOn:
		fmt.Println("Scanning...")
		d.Scan([]gatt.UUID{}, false)
		return
	default:
		d.StopScanning()
	}
}

func onPeriphDiscovered(p gatt.Peripheral, a *gatt.Advertisement, rssi int) {
	if strings.ToUpper(a.LocalName) != "MJ_HT_V1" {
		return
	}

	// Stop scanning once we've got the peripheral we're looking for.
	p.Device().StopScanning()

	fmt.Printf("\nPeripheral ID:%s, NAME:(%s)\n", p.ID(), p.Name())
	fmt.Println("  Local Name        =", a.LocalName)
	fmt.Println("  TX Power Level    =", a.TxPowerLevel)
	fmt.Println("  Manufacturer Data =", a.ManufacturerData)
	fmt.Println("  Service Data      =", a.ServiceData)
	fmt.Println("")

	p.Device().Connect(p)
}

func onPeriphConnected(p gatt.Peripheral, err error) {
	fmt.Println("Connected")
	defer p.Device().CancelConnection(p)

	if err := p.SetMTU(500); err != nil {
		fmt.Printf("Failed to set MTU, err: %s\n", err)
	}

	// Discovery services
	ss, err := p.DiscoverServices(nil)
	if err != nil {
		fmt.Printf("Failed to discover services, err: %s\n", err)
		return
	}

	for _, s := range ss {
		//msg := "Service: " + s.UUID().String()
		//if len(s.Name()) > 0 {
		//	msg += " (" + s.Name() + ")"
		//}
		//fmt.Println(msg)

		// Discovery characteristics
		cs, err := p.DiscoverCharacteristics(nil, s)
		if err != nil {
			fmt.Printf("Failed to discover characteristics, err: %s\n", err)
			continue
		}

		for _, c := range cs {
			//msg := "  Characteristic  " + c.UUID().String()
			//if len(c.Name()) > 0 {
			//	msg += " (" + c.Name() + ")"
			//}
			//msg += "\n    properties    " + c.Properties().String()
			//fmt.Println(msg)

			// Read the characteristic, if possible.
			if (c.Properties() & gatt.CharRead) != 0 {
				//b, err := p.ReadCharacteristic(c)
				//if err != nil {
				//	fmt.Printf("Failed to read characteristic, err: %s\n", err)
				//	continue
				//}
				//fmt.Printf("    value         %x | %q\n", b, b)
			}

			// Discovery descriptors
			_, err := p.DiscoverDescriptors(nil, c)
			if err != nil {
				fmt.Printf("Failed to discover descriptors, err: %s\n", err)
				continue
			}

			//for _, d := range ds {
			//	msg := "  Descriptor      " + d.UUID().String()
			//	if len(d.Name()) > 0 {
			//		msg += " (" + d.Name() + ")"
			//	}
			//	fmt.Println(msg)
			//
			//	// Read descriptor (could fail, if it's not readable)
			//	b, err := p.ReadDescriptor(d)
			//	if err != nil {
			//		fmt.Printf("Failed to read descriptor, err: %s\n", err)
			//		continue
			//	}
			//	fmt.Printf("    value         %x | %q\n", b, b)
			//}

			// Subscribe the characteristic, if possible.
			if (c.Properties() & (gatt.CharNotify | gatt.CharIndicate)) != 0 {
				f := func(c *gatt.Characteristic, b []byte, err error) {
					//fmt.Printf("notified: % X | %q\n", b, b)

					// todo: check pattern
					T, H := b[2:6], b[9:13]
					fmt.Printf("%s ", time.Now().UTC())
					if temperature, err := strconv.ParseFloat(string(T), 32); err == nil {
						fmt.Printf("temperature %.1f, ", temperature)
					}
					if humidity, err := strconv.ParseFloat(string(H), 32); err == nil {
						fmt.Printf("humidity %.1f ", humidity)
					}
					fmt.Println()
				}
				if err := p.SetNotifyValue(c, f); err != nil {
					fmt.Printf("Failed to subscribe characteristic, err: %s\n", err)
					continue
				}

				//msg := "  Subscribed characteristic  " + c.UUID().String()
				//if len(c.Name()) > 0 {
				//	msg += " (" + c.Name() + ")"
				//}
				//msg += "\n    properties    " + c.Properties().String()
				//fmt.Println(msg)
			}

		}
		//fmt.Println()
	}
	waitSec := 5
	fmt.Printf("Waiting for %d seconds to get some notifiations, if any.\n", waitSec)
	time.Sleep(time.Duration(waitSec) * time.Second)

}

func onPeriphDisconnected(p gatt.Peripheral, err error) {
	p.Device().StopAdvertising()
	name := p.Name()
	fmt.Printf("Disconnected: '%s'.\n", name)
	done <- name
}

func main() {

	d, err := gatt.NewDevice(option.DefaultClientOptions...)
	if err != nil {
		log.Fatalf("Failed to open device, err: %s\n", err)
		return
	}

	// Register handlers.
	d.Handle(
		gatt.PeripheralDiscovered(onPeriphDiscovered),
		gatt.PeripheralConnected(onPeriphConnected),
		gatt.PeripheralDisconnected(onPeriphDisconnected),
	)
	for i := 0; i < 10; i++ {
		d.Init(onStateChanged)

		device := <-done
		fmt.Printf("Device %s is disconnected", device)

		waitSec := 10
		fmt.Printf("Sleep for %d seconds\n", waitSec)
		time.Sleep(time.Duration(waitSec) * time.Second)
	}
	close(done)

	fmt.Println("Done")
}
