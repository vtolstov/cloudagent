package guest_fsresize

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/vtolstov/go-ioctl"
)

func resizefs(path string) error {
	var err error
	var stdout io.ReadCloser
	var stdin bytes.Buffer

	partstart := 0
	partnum := 0
	device := "/tmp/resize_dev"
	partition := "/tmp/resize_part"
	active := false
	extended := false
	parttype := "Linux"
	devFs, err := findFs(path)
	if err != nil {
		return err
	}
	devBlk, err := findBlock("/sys/block", devFs)
	if err != nil {
		return err
	}
	os.Remove(device)
	if err = syscall.Mknod(device, uint32(os.ModeDevice|syscall.S_IFBLK|0600), devBlk.Int()); err != nil {
		return err
	}
	defer os.Remove(device)
	//	mbr := make([]byte, 446)

	/*
		f, err := os.OpenFile(device, os.O_RDONLY, os.FileMode(0400))
		if err != nil {
			return err
		}
		_, err = io.ReadFull(f, mbr)
		f.Close()
		if err != nil {
			return err
		}
	*/
	cmd := exec.Command("fdisk", "-l", "-u", device)
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		log.Printf("failed to open %s via fdisk %s 2\n", device, err.Error())
		return err
	}
	r := bufio.NewReader(stdout)

	if err = cmd.Start(); err != nil {
		log.Printf("failed to open %s via fdisk %s 3\n", device, err.Error())
		return err
	}

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}

		if strings.HasPrefix(line, device) {
			partnum++
			///test3        *  16384 204799  188416  92M  5 Extended
			ps := strings.Fields(line)
			if ps[1] == "*" {
				active = true
				partstart, _ = strconv.Atoi(ps[2])
				if len(ps) > 7 {
					parttype = ps[6]
					if ps[7] == "Extended" {
						extended = true
					}
				} else {
					parttype = ps[5]
					if ps[6] == "Extended" {
						extended = true
					}
				}
			} else {
				active = false
				partstart, _ = strconv.Atoi(ps[1])
				if len(ps) > 6 {
					parttype = ps[5]
					if ps[6] == "Extended" {
						extended = true
					}
				} else {
					parttype = ps[4]
					if ps[5] == "Extended" {
						extended = true
					}
				}
			}
		}
	}

	if err = cmd.Wait(); err != nil || partstart == 0 {
		return fmt.Errorf("failed to open %s via fdisk 4\n", device)
	}
	if partnum > 1 {
		stdin.Write([]byte("d\n" + fmt.Sprintf("%d", partnum) + "\n"))
	} else {
		stdin.Write([]byte("d\n"))
	}
	if extended {
		stdin.Write([]byte("n\nl\n" + fmt.Sprintf("%d", partnum) + "\n" + fmt.Sprintf("%d", partstart) + "\n\n"))
	} else {
		stdin.Write([]byte("n\np\n" + fmt.Sprintf("%d", partnum) + "\n" + fmt.Sprintf("%d", partstart) + "\n\n"))
	}
	if active {
		stdin.Write([]byte("a\n" + fmt.Sprintf("%d", partnum) + "\n"))
	}
	if partnum > 1 {
		stdin.Write([]byte("t\n" + fmt.Sprintf("%d", partnum) + "\n" + parttype + "\nw"))
	} else {
		stdin.Write([]byte("t\n" + parttype + "\nw"))
	}
	cmd = exec.Command("fdisk", "-u", device)
	cmd.Stdin = &stdin
	cmd.Run()
	stdin.Reset()

	w, err := os.OpenFile(device, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	/*
		_, err = w.Write(mbr)
		if err != nil {
			return err
		}
		err = w.Sync()
		if err != nil {
			return err
		}
	*/
	blkerr := ioctl.BlkRRPart(w.Fd())
	err = w.Close()
	if err != nil {
		return err
	}
	if blkerr != nil {
		args := []string{}
		for _, name := range []string{"partx", "partprobe", "kpartx"} {
			if _, err = exec.LookPath(name); err == nil {
				switch name {
				case "partx":
					args = []string{"-u", device}
				default:
					args = []string{device}
				}
				log.Printf("update partition table via %s %s", name, strings.Join(args, " "))
				if err = exec.Command(name, args...).Run(); err == nil {
					break
				}
			}
		}
	}
	os.Remove(partition)
	if err = syscall.Mknod(partition, uint32(os.ModeDevice|syscall.S_IFBLK|0600), devFs.Int()); err != nil {
		return err
	}
	defer os.Remove(partition)
	log.Printf("resize filesystem via %s %s", "resize2fs", partition)
	buf, err := exec.Command("resize2fs", partition).CombinedOutput()
	if err != nil {
		log.Printf("resize2fs %s", buf)
		return err
	}
	return nil
}

type dev struct {
	Major uint64
	Minor uint64
}

func (d *dev) String() string {
	return fmt.Sprintf("%d:%d", d.Major, d.Minor)
}

func (d *dev) Int() int {
	return int(d.Major*256 + d.Minor)
}

func findFs(path string) (*dev, error) {
	var st syscall.Stat_t

	err := syscall.Stat(path, &st)
	if err != nil {
		return nil, err
	}
	return &dev{Major: uint64(st.Dev / 256), Minor: uint64(st.Dev % 256)}, nil
}

func findBlock(start string, s *dev) (*dev, error) {
	var err error
	fis, err := ioutil.ReadDir(start)
	if err != nil {
		return nil, err
	}
	for _, fi := range fis {
		switch fi.Name() {
		case "bdi", "subsystem", "device", "trace":
			continue
		}
		if _, err := os.Stat(filepath.Join(start, "dev")); err == nil {
			if buf, err := ioutil.ReadFile(filepath.Join(start, "dev")); err == nil {
				devstr := strings.TrimSpace(string(buf))
				if s.String() == devstr {
					if buf, err = ioutil.ReadFile(filepath.Join(filepath.Dir(start), "dev")); err == nil {
						majorminor := strings.Split(strings.TrimSpace(string(buf)), ":")
						major, _ := strconv.Atoi(majorminor[0])
						minor, _ := strconv.Atoi(majorminor[1])
						return &dev{Major: uint64(major), Minor: uint64(minor)}, nil
					}
				}
			}
		}
		devBlk, err := findBlock(filepath.Join(start, fi.Name()), s)
		if err == nil {
			return devBlk, err
		}
	}
	return nil, errors.New("failed to find dev")
}
