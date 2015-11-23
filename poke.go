package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"syscall"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	pid := getPid()
	searchVal := getSearchVal()
	attachToProcess(pid)
	matchingAddresses := searchRegions(searchVal, pid)
	for len(matchingAddresses) > 1 {
		fmt.Println("num matches:", len(matchingAddresses))
		resumeProcess(pid)
		searchVal = getSearchVal()
		stopProcess(pid)
		matchingAddresses = searchOldMatches(searchVal, matchingAddresses, pid)
	}
	if len(matchingAddresses) == 1 {
		fmt.Println("found a single match!")
		replaceVal := getReplacementValue()
		pokeData(pid, replaceVal, matchingAddresses[0])
	} else {
		fmt.Println("no matches found")
	}
	detach(pid)
}

func getPid() int {
	var pid int
	getIntFromUser("target pid: ", &pid)
	return pid
}

func getSearchVal() []byte {
	var val int32
	getIntFromUser("value to find: ", &val)
	return intToBytes(val)
}

func getReplacementValue() []byte {
	var val int32
	getIntFromUser("replacement value: ", &val)
	return intToBytes(val)
}

func getIntFromUser(prompt string, i interface{}) {
	for true {
		fmt.Print(prompt)
		_, err := fmt.Scanf("%d", i)
		if err != nil {
			fmt.Println(err)
		} else {
			break
		}
	}
}

func attachToProcess(pid int) {
	err := syscall.PtraceAttach(pid)
	if err != nil {
		panic(err)
	}
	waitForStop(pid)
	// replace with optional logging?
	fmt.Println("successfully attached to", pid)
}

func waitForStop(pid int) {
	var status syscall.WaitStatus
	_, err := syscall.Wait4(pid, &status, 0, nil)
	if err != nil || !status.Stopped() {
		// LOG
		fmt.Println("target didn't stop")
	}
}

func detach(pid int) {
	fmt.Println("detaching from", pid)
	err := syscall.PtraceDetach(pid)
	if err != nil {
		panic(err)
	}
	// replace with optional logging?
	fmt.Println("detached from", pid)
}

func pokeData(pid int, dataBytes []byte, addr int64) {
	fmt.Println("replacing with value:", bytesToInt(dataBytes))
	_, err := syscall.PtracePokeData(pid, uintptr(addr), dataBytes)
	if err != nil {
		fmt.Println("unable to write data")
	}
}

func resumeProcess(pid int) {
	err := syscall.PtraceCont(pid, 0)
	if err != nil {
		panic(err)
	}
}

func stopProcess(pid int) {
	err := syscall.Kill(pid, syscall.SIGSTOP)
	if err != nil {
		panic(err)
	}
	waitForStop(pid)
}

func searchOldMatches(val []byte, oldMatches []int64, pid int) []int64 {
	var matches []int64
	mem := openMemFile(pid)
	defer mem.Close()
	matches = appendMatches(val, matches, oldMatches, mem)
	return matches
}

func appendMatches(val []byte, matches, oldMatches []int64, mem *os.File) []int64 {
	buf := make([]byte, len(val))
	for _, addr := range oldMatches {
		mem.Seek(addr, 0)
		current := fill(buf, mem)
		if bytes.Equal(val, current) {
			matches = append(matches, addr)
		}
	}
	return matches
}

type Region struct {
	start, end int64
}

func searchRegions(val []byte, pid int) []int64 {
	var matches []int64
	regions := getWritableRegions(pid)
	mem := openMemFile(pid)
	defer mem.Close()
	for _, region := range regions {
		matches = appendRegionMatches(val, matches, region, mem)
	}
	return matches
}

func openMemFile(pid int) *os.File {
	file, err := os.Open("/proc/" + strconv.Itoa(pid) + "/mem")
	if err != nil {
		panic(err)
	}
	return file
}

func appendRegionMatches(val []byte, matches []int64, region Region, mem *os.File) []int64 {
	var bufSize int64 = 4096
	buf := make([]byte, bufSize)
	mem.Seek(region.start, 0)
	for offset := region.start; offset < region.end; offset += bufSize {
		segmentLen := min(bufSize, region.end-offset)
		matches = appendSegmentMatches(val, matches, offset, fill(buf[:segmentLen], mem))
	}
	return matches
}

func appendSegmentMatches(val []byte, matches []int64, position int64, segment []byte) []int64 {
	size := len(val)
	for offset := 0; offset < len(segment); offset += size {
		if bytes.Equal(val, segment[offset:offset+size]) {
			matches = append(matches, position+int64(offset))
		}
	}
	return matches
}

func fill(buf []byte, mem *os.File) []byte {
	_, err := io.ReadFull(mem, buf)
	if err != nil {
		panic(err)
	}
	return buf
}

func intToBytes(data int32) []byte {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.LittleEndian, data)
	if err != nil {
		fmt.Println(err)
	}
	return buf.Bytes()
}

func bytesToInt(data []byte) int32 {
	var result int32
	err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian, &result)
	if err != nil {
		fmt.Println(err)
	}
	return result
}

func getWritableRegions(pid int) []Region {
	var regions []Region
	file, err := os.Open("/proc/" + strconv.Itoa(pid) + "/maps")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		region := getRegionIfMatch(line)
		if region != nil {
			regions = append(regions, *region)
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	return regions
}

var memSegRE = regexp.MustCompile(`([\da-f]+)-([\da-f]+) +.w.. +.*`)

func getRegionIfMatch(line string) *Region {
	matches := memSegRE.FindStringSubmatch(line)
	if matches != nil {
		var region Region
		region.start, _ = strconv.ParseInt(matches[1], 16, 64)
		region.end, _ = strconv.ParseInt(matches[2], 16, 64)
		return &region
	}
	return nil
}

func min(a, b int64) int64 {
	if a < b {
		return a
	} else {
		return b
	}
}
