package main

import "bufio"
import "fmt"
import "os"
import "regexp"
import "strconv"
import "syscall"

func main() {
  pid := getPid()
  search_val := getSearchVal()
  attachToProcess(pid)
  matchingAddresses := search(search_val, pid)
  fmt.Println(len(matchingAddresses))
  detach(pid)
  // get new search value
  // reattach
  // search target mem again
  // repeat until a single match is found
  // then poke with new value
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
  // replace with optional logging?
  fmt.Println("successfully attached to", pid)
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

type Region struct {
  start, end int64
}

func search(val int64, pid int) []int64 {
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
  segment := getBytes(region.start, region.end, mem)
  size := 4
  for addr := 0; addr < len(segment); addr += size {
    current := bytesToInt(segment[addr:addr + size])
    if current == val {
      matches = append(matches, region.start + int64(addr))
    }
  }
  return matches
}

func getBytes(start, end int64, mem *os.File) []byte {
  result := make([]byte, end - start)
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

func bytesToInt(bytes []byte) int64 {
  // TODO handle different sizes -- hardcoded to 4 now
  return int64(bytes[0]) | int64(bytes[1]) << 8 | int64(bytes[2]) << 16 | int64(bytes[3]) << 24
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
