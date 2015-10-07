package main

import (
  "archive/zip"
  "bufio"
  "fmt"
  "io"
  "io/ioutil"
  "log"
  "math/rand"
  "net/http"
  "os"
  "os/exec"
  "path"
  "path/filepath"
  "runtime"
  "time"
  "github.com/magiconair/properties"
  "github.com/mitchellh/go-homedir"
)

const ARCH_BITS = 32 + int(^uintptr(0)>>63<<5)
const RisePlayerShutdownURL = "http://localhost:9449/shutdown"
const RiseCacheShutdownURL = "http://localhost:9494/shutdown"

var config *properties.Properties
var remoteURLs = []string{ "BrowserURL", "CacheURL", "JavaURL", "PlayerURL" }
var remoteVersions = []string{ "BrowserVersion", "CacheVersion", "JavaVersion", "PlayerVersion" }
var localVersions = []string{ "chromium", "RiseCache", "java", "RisePlayer" }
var destinationDir = []string{ "", "RiseCache", "JRE", "" }
var channel = "Stable"

func main() {
  performInstallation()
}

func performInstallation() {
  config = loadRemoteComponents()

  if !config.MustGetBool("ForceStable") && int(rand.Float32() * 100) < config.MustGetInt("LatestRolloutPercent") {
    channel = "Latest"
  }

  for idx, _ := range remoteVersions {
    remoteVersion := config.MustGetString(remoteVersions[idx] + channel)
    localVersion := getLocalVersion(localVersions[idx])

    if localVersion != remoteVersion {
      fmt.Println("Downloading: " + config.MustGetString(remoteURLs[idx] + channel))
      downloadComponent(remoteURLs[idx] + channel, config)
    }
  }

  err := os.MkdirAll(getInstallDir(), 0755)

  if err != nil {

  }

  for idx, _ := range remoteVersions {
    remoteVersion := config.MustGetString(remoteVersions[idx] + channel)
    localVersion := getLocalVersion(localVersions[idx])

    if localVersion != remoteVersion {
      fmt.Println("Extracting: " + path.Base(config.MustGetString(remoteURLs[idx] + channel)))
      extractComponent(remoteURLs[idx] + channel, destinationDir[idx], config)
      saveLocalVersion(localVersions[idx], remoteVersion)
    }
  }

  fixChromiumDirectoryName()

  if _, err := os.Stat(getConfigFileName()); err != nil {
    createConfigFile("", "", "https://rvacore-test.appspot.com", "http://rvaviewer-test.appspot.com")
  }

  fmt.Println("Shutting down Cache")
  _, err = http.Get(RiseCacheShutdownURL)
  time.Sleep(2000)

  fmt.Println("Shutting down Player")
  _, err = http.Get(RisePlayerShutdownURL)
  time.Sleep(2000)

  fmt.Println("Starting Cache")
  startJavaProcess(path.Join("RiseCache", "RiseCache.jar"))

  fmt.Println("Starting Player")
  startJavaProcess(path.Join("RisePlayer.jar"))

  fmt.Println("Process finished")
}

func startJavaProcess(jarPath string) (error) {
  javaPath := path.Join(getInstallDir(), "JRE", "bin", "javaw.exe")
  cmd := exec.Command(javaPath, "-jar", filepath.FromSlash(path.Join(getInstallDir(), jarPath)))
  err := cmd.Start()

  if err != nil {
    log.Fatal(err)
  }

  return err
}

func getConfigFileName() (string) {
  return path.Join(getInstallDir(), "RiseDisplayNetworkII.ini")
}

func createConfigFile(displayID string, claimID string, coreURL string, viewerURL string) {
  var contents = "[RDNII]\n"

  contents += "displayid=" + displayID + "\n"
  contents += "claimid=" + claimID + "\n"
  contents += "coreurl=" + coreURL + "\n"
  contents += "viewerurl=" + viewerURL + "\n"

  ioutil.WriteFile(getConfigFileName(), []byte(contents), 0755)
}

func getInstallDir() string {
  dir, err := homedir.Dir();

  if err != nil {

  }

  if runtime.GOOS == "windows" {
    return dir + "\\AppData\\Local\\RVPlayer2"
  } else {
    return dir + "/rvplayer2"
  }
}

func getLocalVersion(name string) string {
  path := path.Join(getInstallDir(), name + ".ver")
  inFile, _ := os.Open(path)
  defer inFile.Close()
  scanner := bufio.NewScanner(inFile)
  scanner.Split(bufio.ScanLines) 
  
  scanner.Scan()
  return scanner.Text()
}

func saveLocalVersion(name string, data string) (error) {
  return ioutil.WriteFile(path.Join(getInstallDir(), name + ".ver"), []byte(data), 0755)
}

func getTempFileName(name string, config *properties.Properties) (string) {
  return path.Join(os.TempDir(), path.Base(config.MustGetString(name)))
}

func fetchURLContent(url string) ([]byte, error) {
  resp, err := http.Get(url)

  if err != nil {
    return nil, err
  }

  defer resp.Body.Close()

  return ioutil.ReadAll(resp.Body)
}

func downloadComponent(name string, config *properties.Properties) (error) {
  data, err := fetchURLContent(config.MustGetString(name))

  if err != nil {
    return err
  }

  return ioutil.WriteFile(getTempFileName(name, config), data, 0755)
}

func extractComponent(name string, destinationDir string, config *properties.Properties) {
  unzip(getTempFileName(name, config), path.Join(getInstallDir(), destinationDir))
}

func loadRemoteComponents() (*properties.Properties) {
  var configFile string

  if runtime.GOOS == "windows" {
    configFile = "win"
  } else if runtime.GOOS == "linux" && ARCH_BITS == 32 {
    configFile = "lnx-32"
  } else if runtime.GOOS == "linux" && ARCH_BITS == 64 {
    configFile = "lnx-64"
  } else {
    panic("Platform not supported")
  }

  body, err := fetchURLContent("http://install-versions.risevision.com/remote-components-" + configFile + ".cfg")

  if err != nil {
    fmt.Println("Error loading remote components file")
  }

  props, err := properties.Load(body, properties.UTF8)

  if err != nil {
    fmt.Println("Error reading remote components file")
  }

  return props
}

func fixChromiumDirectoryName() {
  var dirName = "win32"

  if runtime.GOOS == "linux" {
    dirName = "linux"
  }

  os.Rename(path.Join(getInstallDir(), "chrome-" + dirName), path.Join(getInstallDir(), "chromium"))
}

func unzip(archive, target string) error {
  reader, err := zip.OpenReader(archive)
  if err != nil {
    log.Fatal("a", err)
    return err
  }

  if err := os.MkdirAll(target, 0755); err != nil {
    log.Fatal("b", err)
    return err
  }

  for _, file := range reader.File {
    filePath := path.Join(target, file.Name)
    if file.FileInfo().IsDir() {
      os.MkdirAll(filePath, file.Mode())
      continue
    } else {
      fmt.Println("Creating: " + filePath + " " + path.Dir(filePath))
      os.MkdirAll(path.Dir(filePath), file.Mode())
    }

    fileReader, err := file.Open()
    if err != nil {
      log.Fatal("c", err)
      return err
    }
    defer fileReader.Close()

    targetFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
    if err != nil {
      log.Fatal("d", err)
      return err
    }
    defer targetFile.Close()

    if _, err := io.Copy(targetFile, fileReader); err != nil {
      log.Fatal("e", err)
      return err
    }
  }

  return nil
}
