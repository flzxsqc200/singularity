package runtime

/*
#include <unistd.h>
#include "image/image.h"
#include "util/config_parser.h"
*/
// #cgo CFLAGS: -I../c
// #cgo LDFLAGS: -lsycore -luuid
import "C"

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"path"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/singularityware/singularity/internal/pkg/runtime/engine/singularity/config"
	"github.com/singularityware/singularity/internal/pkg/runtime/engine/singularity/rpc/client"
	"github.com/singularityware/singularity/pkg/util/loop"
)

func (engine *RuntimeEngine) CreateContainer(rpcConn net.Conn) error {
	if engine.RuntimeSpec.RuntimeName != config.Name {
		log.Fatalln("engineName configuration doesn't match runtime name")
	}
	rpcOps := &client.Rpc{rpc.NewClient(rpcConn), engine.RuntimeSpec.RuntimeName}
	if rpcOps.Client == nil {
		log.Fatalln("Failed to initialiaze RPC client")
	}

	_, err := rpcOps.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	if err != nil {
		log.Fatalln("mount / failed:", err)
	}

	st, err := os.Stat(engine.OciConfig.RuntimeOciSpec.Root.Path)
	if err != nil {
		log.Fatalf("stat on %s failed\n", engine.OciConfig.RuntimeOciSpec.Root.Path)
	}

	rootfs := engine.OciConfig.RuntimeOciSpec.Root.Path

	userNS := false
	pidNS := false

	for _, namespace := range engine.OciConfig.RuntimeOciSpec.Linux.Namespaces {
		switch namespace.Type {
		case specs.UserNamespace:
			userNS = true
		case specs.PIDNamespace:
			pidNS = true
		}
	}

	os.Setenv("SINGULARITY_MESSAGELEVEL", os.Getenv("MESSAGELEVEL"))
	C.singularity_config_init()

	imageObject := C.singularity_image_init(C.CString(rootfs), 0)

	info := new(loop.LoopInfo64)
	mountType := ""

	switch C.singularity_image_type(&imageObject) {
	case 1:
		mountType = "squashfs"
		info.Offset = uint64(C.uint(imageObject.offset))
		info.SizeLimit = uint64(C.uint(imageObject.size))
	case 2:
		mountType = "ext3"
		info.Offset = uint64(C.uint(imageObject.offset))
		info.SizeLimit = uint64(C.uint(imageObject.size))
	}

	if st.IsDir() == false && !userNS {
		var number int
		info.Flags = loop.FlagsAutoClear
		number, err = rpcOps.LoopDevice(rootfs, os.O_RDONLY, *info)
		if err != nil {
			fmt.Println(err)
		}
		path := fmt.Sprintf("/dev/loop%d", number)
		rootfs = "/tmp/testing"
		_, err = rpcOps.Mount(path, rootfs, mountType, syscall.MS_NOSUID|syscall.MS_RDONLY|syscall.MS_NODEV, "errors=remount-ro")
		if err != nil {
			fmt.Println("mount "+mountType, err)
		}
	}

	if pidNS {
		_, err = rpcOps.Mount("proc", path.Join(rootfs, "proc"), "proc", syscall.MS_NOSUID, "")
		if err != nil {
			log.Fatalln("mount proc failed:", err)
		}
	} else {
		_, err = rpcOps.Mount("/proc", path.Join(rootfs, "proc"), "", syscall.MS_BIND|syscall.MS_NOSUID|syscall.MS_REC, "")
		if err != nil {
			log.Fatalln("mount proc failed:", err)
		}
	}
	if !userNS {
		_, err = rpcOps.Mount("sysfs", path.Join(rootfs, "sys"), "sysfs", syscall.MS_NOSUID, "")
		if err != nil {
			log.Fatalln("mount sys failed:", err)
		}
	} else {
		_, err = rpcOps.Mount("/sys", path.Join(rootfs, "sys"), "", syscall.MS_BIND|syscall.MS_NOSUID|syscall.MS_REC, "")
		if err != nil {
			log.Fatalln("mount sys failed:", err)
		}
	}

	_, err = rpcOps.Mount("/dev", path.Join(rootfs, "dev"), "", syscall.MS_BIND|syscall.MS_NOSUID|syscall.MS_REC, "")
	if err != nil {
		log.Fatalln("mount dev failed:", err)
	}
	_, err = rpcOps.Mount("/etc/passwd", path.Join(rootfs, "etc/passwd"), "", syscall.MS_BIND, "")
	if err != nil {
		log.Fatalln("mount /etc/passwd failed:", err)
	}
	_, err = rpcOps.Mount("/etc/group", path.Join(rootfs, "etc/group"), "", syscall.MS_BIND, "")
	if err != nil {
		log.Fatalln("mount /etc/group failed:", err)
	}
	_, err = rpcOps.Mount(rootfs, "/mnt", "", syscall.MS_BIND|syscall.MS_REC, "")
	if err != nil {
		log.Fatalln("mount failed:", err)
	}
	err = syscall.Chdir("/mnt")
	if err != nil {
		log.Fatalln("change directory failed:", err)
	}
	_, err = rpcOps.Chroot("/mnt")
	if err != nil {
		log.Fatalln("chroot failed:", err)
	}
	err = syscall.Chdir("/")
	if err != nil {
		log.Fatalln("change directory failed:", err)
	}
	if err := rpcOps.Client.Close(); err != nil {
		log.Fatalln("Can't close connection with rpc server")
	}

	return nil
}
