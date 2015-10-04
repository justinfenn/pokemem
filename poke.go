package main

import "bufio"
import "fmt"
import "os"
import "regexp"
import "runtime"
import "strconv"
import "syscall"

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

func getSearchVal() int64 {
	var val int64
	getIntFromUser("value to find: ", &val)
	return val
}

func getReplacementValue() int {
	var val int
	getIntFromUser("replacement value: ", &val)
	return val
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

func pokeData(pid, data int, addr int64) {
	dataBytes := intToBytes(data)
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

func searchOldMatches(val int64, oldMatches []int64, pid int) []int64 {
	var matches []int64
	mem := openMemFile(pid)
	defer mem.Close()
	matches = appendMatches(val, matches, oldMatches, mem)
	return matches
}

func appendMatches(val int64, matches, oldMatches []int64, mem *os.File) []int64 {
	var size int64 = 4 // TODO replace with global or user value
	for _, addr := range oldMatches {
		bytes := getBytes(addr, size, mem)
		current := bytesToInt(bytes)
		if current == val {
			matches = append(matches, addr)
		}
	}
	return matches
}

type Region struct {
	start, end int64
}

func searchRegions(val int64, pid int) []int64 {
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

func appendRegionMatches(val int64, matches []int64, region Region, mem *os.File) []int64 {
	segment := getBytes(region.start, region.end-region.start, mem)
	size := 4
	for addr := 0; addr < len(segment); addr += size {
		current := bytesToInt(segment[addr : addr+size])
		if current == val {
			matches = append(matches, region.start+int64(addr))
		}
	}
	return matches
}

func getBytes(start, length int64, mem *os.File) []byte {
	result := make([]byte, length)
	mem.Seek(start, 0)
	totalBytesRead := 0
	for totalBytesRead < len(result) {
		bytesRead, err := mem.Read(result[totalBytesRead:])
		if err != nil {
			panic(err)
		}
		totalBytesRead += bytesRead
	}
	return result
}

func intToBytes(data int) []byte {
	bytes := make([]byte, 4)
	bytes[0] = byte(data)
	bytes[1] = byte(data >> 8)
	bytes[2] = byte(data >> 16)
	bytes[3] = byte(data >> 24)
	return bytes
}

func bytesToInt(bytes []byte) int64 {
	// TODO handle different sizes -- hardcoded to 4 now
	return int64(bytes[0]) | int64(bytes[1])<<8 | int64(bytes[2])<<16 | int64(bytes[3])<<24
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

//var memSegRE = regexp.MustCompile(`([\da-f]+)-([\da-f]+) +.w.. +[^\s]+ +[^\s]+ +0 +.*`)
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
