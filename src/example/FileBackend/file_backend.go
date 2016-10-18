package main

import (
	"fmt"
	"os"
	"os/signal"

	"libtcmu"
	//"time"
	"runtime"
	"syscall"
	//"time"

	"github.com/Sirupsen/logrus"
)

var (
	fvol1 *os.File
	fvol2 *os.File
)

func test(hba *tcmu.HBA) {
	filename := os.Args[2]
	f, err := os.OpenFile(filename, os.O_RDWR, 0700)
	if err != nil {
		die("couldn't open: %v", err)
	}
	defer f.Close()
	fi, _ := f.Stat()

	d, err := hba.CreateDevice(fi.Name(), 1073741824, 1024, f)
	if err != nil {
		die("couldn't tcmu: %v", err)
	}
	defer d.Close()
	fmt.Printf("go-tcmu attached to %s/%s\n", "/dev/tcmufile", fi.Name())
	//time.Sleep(time.Second)
	//d.GenerateDevEntry()
	mainClose := make(chan bool)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	go func() {
		for _ = range signalChan {
			fmt.Println("\n[test1] Received an interrupt, stopping services...")
			close(mainClose)
		}
	}()
	<-mainClose
}

func Create(name string, hba *tcmu.HBA) {
	var err error
	fvol1, err = os.OpenFile(name, os.O_RDWR, 0700)
	if err != nil {
		die("couldn't open: %v", err)
	}
	//defer fvol1.Close()
	fi, _ := fvol1.Stat()

	_, err = hba.CreateDevice(fi.Name(), fi.Size(), 1024, fvol1)
	if err != nil {
		die("couldn't tcmu: %v", err)
	}
}

func Close(name string, hba *tcmu.HBA) {
	hba.RemoveDevice(name)
	fvol1.Close()
	fvol1 = nil
}

func mainRoutine() {
	hba, _ := tcmu.NewHBA("tcomet")
	hba.Start()

	go test(hba)

	mainClose := make(chan bool)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	go func() {
		for _ = range signalChan {
			fmt.Println("\nReceived an interrupt, stopping services...")
			close(mainClose)
		}
	}()
	<-mainClose
}

func CreateOne() {
	hba, _ := tcmu.NewHBA("tcomet")
	hba.Start()
	filename := os.Args[2]

	Create(filename, hba)

	mainClose := make(chan bool)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, os.Kill, syscall.SIGTERM)

	go func() {
		for _ = range signalChan {
			fmt.Println("\n[main] Received an interrupt, stopping services...")
			Close(filename, hba)
			close(mainClose)
		}
	}()
	<-mainClose
}

func CreateMany() {
	hba, _ := tcmu.NewHBA("tcomet")
	hba.Start()
	filename := os.Args[2]

	mainClose := make(chan bool)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, os.Kill, syscall.SIGTERM)

	go func() {
		for _ = range signalChan {
			buf := make([]byte, 8192*4)
			runtime.Stack(buf, true)
			fmt.Println("\n[main] Received an interrupt, stopping services...num:", runtime.NumGoroutine())
			//fmt.Println(string(buf))
			Close(filename, hba)
			close(mainClose)
		}
	}()

	for i := 0; i < 20000; i++ {
		fmt.Printf("Times: %d\n", i)
		Create(filename, hba)
		Close(filename, hba)
	}

	<-mainClose
}

func die(why string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, why+"\n", args...)
	os.Exit(1)
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	if len(os.Args) < 3 {
		return
	}

	if os.Args[1] == "once" {
		CreateOne()
	}

	if os.Args[1] == "many" {
		CreateMany()
	}

	if os.Args[1] == "clear" && len(os.Args) == 3 {

	}
}

/*
go build file_backend.go

truncate -s 1G /home/vol2

modprobe target_core_user

./file_backend once

mount -t tmpfs -o size=1400m tmpfs /tmp2
truncate -s 1G /tmp2/vol2

dd if=/dev/zero of=/dev/tcomet/vol2 bs=4K count=262144
*/
