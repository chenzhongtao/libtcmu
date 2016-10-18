package tcmu

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

/*
内核代码在 target_core_user.h 中定义
https://github.com/torvalds/linux/blob/v4.4/include/uapi/linux/target_core_user.h

#define ALIGN_SIZE 64 // Should be enough for most CPUs

enum tcmu_opcode {
	TCMU_OP_PAD = 0,
	TCMU_OP_CMD,
};

struct tcmu_mailbox {
__u16 version;
__u16 flags;
__u32 cmdr_off;
__u32 cmdr_size;

__u32 cmd_head;

// Updated by user. On its own cacheline
__u32 cmd_tail __attribute__((__aligned__(ALIGN_SIZE)));

} __packed;

struct tcmu_cmd_entry_hdr {
	__u32 len_op; //tcmu_cmd_entry 大小 8字节对齐， 后面三位用来存放tcmu_opcode
	__u16 cmd_id;
	__u8 kflags;
#define TCMU_UFLAG_UNKNOWN_OP 0x1
	__u8 uflags;

} __packed;

#define TCMU_SENSE_BUFFERSIZE 96

struct tcmu_cmd_entry {
	struct tcmu_cmd_entry_hdr hdr;

	union {
		struct {
			uint32_t iov_cnt; //iov_cnt包含了iov[] entries的数量，需要区分Data-In还是Data-Out的缓冲。
			                  //对于双向的command，iov_cnt指定多少iovec entries覆盖了Data-Out区域，
			                  //iov_bidi_cnt指定了多少iovec entries覆盖了Data-In区域（紧接在Data-Out区域）
			uint32_t iov_bidi_cnt;
			uint32_t iov_dif_cnt;
			uint64_t cdb_off; // 用户空间通过tcmu_cmd_entry.req.cdb_off找到SCSI CDB（Command Data Block）
			uint64_t __pad1;
			uint64_t __pad2;
			struct iovec iov[0];  // 0个或多个   零长度数组的妙用   http://blog.chinaunix.net/uid-20196318-id-28810.html
		} req;
		struct {
			uint8_t scsi_status; // 当command执行完成，用户空间设置rsp.scsi_status，如果有需要也设置rsp.sense_buffer
			uint8_t __pad1;
			uint16_t __pad2;
			uint32_t __pad3;
			char sense_buffer[TCMU_SENSE_BUFFERSIZE];
		} rsp;
	};

} __packed;
*/

var byteOrder binary.ByteOrder = binary.LittleEndian

func (vbd *VirBlkDev) mbVersion() uint16 {
	return *(*uint16)(unsafe.Pointer(&vbd.mmap[0]))
}

func (vbd *VirBlkDev) mbFlags() uint16 {
	return *(*uint16)(unsafe.Pointer(&vbd.mmap[2]))
}

// 128,表示Mailbox的大小
func (vbd *VirBlkDev) mbCmdrOffset() uint32 {
	return *(*uint32)(unsafe.Pointer(&vbd.mmap[4]))
}

// command ring区域的大小 65408 前面128字节是 Mailbox
func (vbd *VirBlkDev) mbCmdrSize() uint32 {
	return *(*uint32)(unsafe.Pointer(&vbd.mmap[8]))
}

// 由内核修改，表示一个command已经放置到ring中
func (vbd *VirBlkDev) mbCmdHead() uint32 {
	return *(*uint32)(unsafe.Pointer(&vbd.mmap[12]))
}

// 由用户空间修改，表示一个command已经处理完成
func (vbd *VirBlkDev) mbCmdTail() uint32 {
	return *(*uint32)(unsafe.Pointer(&vbd.mmap[64]))
}

func (vbd *VirBlkDev) mbSetTail(u uint32) {
	byteOrder.PutUint32(vbd.mmap[64:], u)
}

/*
enum tcmu_opcode {
  TCMU_OP_PAD = 0,
  TCMU_OP_CMD,
};
*/
type tcmuOpcode int

const (
	tcmuOpPad tcmuOpcode = 0 //对齐
	tcmuOpCmd            = 1
)

/*

// Only a few opcodes, and length is 8-byte aligned, so use low bits for opcode.
struct tcmu_cmd_entry_hdr {
  __u32 len_op;
  __u16 cmd_id;
  __u8 kflags;
#define TCMU_UFLAG_UNKNOWN_OP 0x1
  __u8 uflags;

} __packed;
*/
func (vbd *VirBlkDev) entHdrOp(off int) tcmuOpcode {
	i := int(*(*uint32)(unsafe.Pointer(&vbd.mmap[off+offLenOp])))
	i = i & 0x7
	return tcmuOpcode(i)
}

func (vbd *VirBlkDev) entHdrGetLen(off int) int {
	i := *(*uint32)(unsafe.Pointer(&vbd.mmap[off+offLenOp]))
	i = i &^ 0x7
	return int(i)
}

func (vbd *VirBlkDev) entCmdId(off int) uint16 {
	return *(*uint16)(unsafe.Pointer(&vbd.mmap[off+offCmdId]))
}
func (vbd *VirBlkDev) setEntCmdId(off int, id uint16) {
	*(*uint16)(unsafe.Pointer(&vbd.mmap[off+offCmdId])) = id
}
func (vbd *VirBlkDev) entKflags(off int) uint8 {
	return *(*uint8)(unsafe.Pointer(&vbd.mmap[off+offKFlags]))
}
func (vbd *VirBlkDev) entUflags(off int) uint8 {
	return *(*uint8)(unsafe.Pointer(&vbd.mmap[off+offUFlags]))
}

func (vbd *VirBlkDev) setEntUflagUnknownOp(off int) {
	vbd.mmap[off+offUFlags] = 0x01
}


func (vbd *VirBlkDev) entReqIovCnt(off int) uint32 {
	return *(*uint32)(unsafe.Pointer(&vbd.mmap[off+offReqIovCnt]))
}

func (vbd *VirBlkDev) entReqIovBidiCnt(off int) uint32 {
	return *(*uint32)(unsafe.Pointer(&vbd.mmap[off+offReqIovBidiCnt]))
}

func (vbd *VirBlkDev) entReqIovDifCnt(off int) uint32 {
	return *(*uint32)(unsafe.Pointer(&vbd.mmap[off+offReqIovDifCnt]))
}

func (vbd *VirBlkDev) entReqCdbOff(off int) uint64 {
	return *(*uint64)(unsafe.Pointer(&vbd.mmap[off+offReqCdbOff]))
}

func (vbd *VirBlkDev) setEntRespSCSIStatus(off int, status byte) {
	vbd.mmap[off+offRespSCSIStatus] = status
}

func (vbd *VirBlkDev) copyEntRespSenseData(off int, data []byte) {
	buf := vbd.mmap[off+offRespSense : off+offRespSense+SENSE_BUFFER_SIZE]
	copy(buf, data)
	if len(data) < SENSE_BUFFER_SIZE {
		for i := len(data); i < SENSE_BUFFER_SIZE; i++ {
			buf[i] = 0
		}
	}
}

func (vbd *VirBlkDev) entIovecN(off int, idx int) []byte {
	out := syscall.Iovec{}
	p := unsafe.Pointer(&vbd.mmap[off+offReqIov0Base])
	out = *(*syscall.Iovec)(unsafe.Pointer(uintptr(p) + uintptr(idx)*unsafe.Sizeof(out)))
	moff := *(*int)(unsafe.Pointer(&out.Base))
	return vbd.mmap[moff : moff+int(out.Len)]
}

func (vbd *VirBlkDev) entCdb(off int) []byte {
	cdbStart := int(vbd.entReqCdbOff(off))
	len := vbd.cdbLen(cdbStart)
	return vbd.mmap[cdbStart : cdbStart+len]
}

func (vbd *VirBlkDev) cdbLen(cdbStart int) int {
	opcode := vbd.mmap[cdbStart]
	// See spc-4 4.2.5.1 operation code
	//
	if opcode <= 0x1f {
		return 6
	} else if opcode <= 0x5f {
		return 10
	} else if opcode == 0x7f {
		return int(vbd.mmap[cdbStart+7]) + 8
	} else if opcode >= 0x80 && opcode <= 0x9f {
		return 16
	} else if opcode >= 0xa0 && opcode <= 0xbf {
		return 12
	} else {
		panic(fmt.Sprintf("what opcode is %x", opcode))
	}
}
