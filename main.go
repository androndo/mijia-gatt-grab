package main

import (
	//"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"strconv"

	//"os"
	"strings"
	"time"

	"github.com/paypal/gatt"
	"github.com/paypal/gatt/examples/option"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var closedDevices = make(chan string)

type SensorMeasurement struct {
	Id string
	CharacteristicName string
	Value float64
}
var measurements = make(chan SensorMeasurement)

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

		// Discovery characteristics
		cs, err := p.DiscoverCharacteristics(nil, s)
		if err != nil {
			fmt.Printf("Failed to discover characteristics, err: %s\n", err)
			continue
		}

		for _, c := range cs {
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

			// Subscribe the characteristic, if possible.
			if (c.Properties() & (gatt.CharNotify | gatt.CharIndicate)) != 0 {
				f := func(c *gatt.Characteristic, b []byte, err error) {
					//fmt.Printf("notified: % X | %q\n", b, b)

					// todo: check pattern
					T, H := b[2:6], b[9:13]
					msg := fmt.Sprintf("%s %s ", time.Now().UTC(), p.ID())
					temperature, errT := strconv.ParseFloat(string(T), 32);

					humidity, errH := strconv.ParseFloat(string(H), 32);
					if errT != nil || errH != nil {
						log.Println("Parsing sensor values was failed")
						return
					}
					measurements <- SensorMeasurement{
						Id: p.ID(),
						CharacteristicName: "temperature",
						Value: temperature,
					}
					measurements <- SensorMeasurement{
						Id: p.ID(),
						CharacteristicName: "humidity",
						Value: humidity,
					}
					fmt.Println(msg)
				}
				if err := p.SetNotifyValue(c, f); err != nil {
					fmt.Printf("Failed to subscribe characteristic, err: %s\n", err)
					continue
				}
			}
		}
	}
	sleepFor(5)
}

func onPeriphDisconnected(p gatt.Peripheral, err error) {
	p.Device().StopAdvertising()
	name := p.Name()
	fmt.Printf("Disconnected: '%s'.\n", name)
	closedDevices <- name
}

func main() {

	sensorMeasurement := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "sensor_measurement",
	},
	[]string{"sensor_id", "characteristic"})
	prometheus.MustRegister(sensorMeasurement)

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":80", nil)

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

	go func() {
		for {
			select {
			case m, ok := <-measurements:
				if ok {
					// Value was read
					sensorMeasurement.WithLabelValues(m.Id, m.CharacteristicName).Set(m.Value)
				} else {
					// Channel closed
					break
				}
			default:
				// No value ready, moving on
				sleepFor(5)
			}
		}
	}()

	for i := 0; i < 10; i++ {
		d.Init(onStateChanged)

		device := <-closedDevices
		fmt.Printf("Device %s is disconnected", device)

		sleepFor(5)
	}

	sleepFor(5)
	close(closedDevices)
	close(measurements)

	fmt.Println("Done")
}

func sleepFor(seconds int) {
	fmt.Printf("Sleep for %d seconds\n", seconds)
	time.Sleep(time.Duration(seconds) * time.Second)
}
