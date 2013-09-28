package main

import "fmt"
import "syscall"

func main() {
  pid := getPid()
  //search_val := getSearchVal()
  attachToPid(pid)
  // read /proc/pid/mem (only values in maps, though)
  // store all value matches
  detach(pid)
  // get new search value
  // reattach
  // search target mem again
  // repeat until a single match is found
  // then poke with new value
}

func getPid() int {
  return getIntFromUser("target pid: ")
}

func getSearchVal() int {
  return getIntFromUser("value to find: ")
}

func getIntFromUser(prompt string) int {
  var i int
  for true {
    fmt.Print(prompt)
    _, err := fmt.Scanf("%d", &i)
    if err != nil {
      fmt.Println(err)
    } else {
      break
    }
  }
  return i
}

func attachToPid(pid int) {
  err := syscall.PtraceAttach(pid)
  if err != nil {
    panic(err)
  }
  // replace with optional logging?
  fmt.Println("successfully attached to", pid)
}

func detach(pid int) {
  err := syscall.PtraceDetach(pid)
  if err != nil {
    panic(err)
  }
  // replace with optional logging?
  fmt.Println("detached from", pid)
}
