package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

func main() {
	targetDirPath := flag.String("d", "", "The directory to search for DLLs")
	fileExtsCsv := flag.String("e", ".dll", "Comma separated list of file extensions to search for")
	outputDirPath := flag.String("o", "", "The output directory. Creates a new directory if not specified")
	respectFileCase := flag.Bool("respect-file-case", false, "Respect filenames' case when matching their extensions")
	noIlspyErrors := flag.Bool("no-ilspy-errors", false, "Exit if ILSpy extraction fails to extract a file")
	scanRecursively := flag.Bool("r", false, "Scan recursively")
	numDecompilers := flag.Int("num-workers", runtime.NumCPU(), "Number of .NET decompiler instances to run concurrently")

	flag.Parse()

	if len(*targetDirPath) == 0 {
		log.Fatal("please specify a target directory path")
	}

	if len(*fileExtsCsv) == 0 {
		log.Fatal("please specify a comma separated list of file extensions")
	}

	if len(*outputDirPath) == 0 {
		*outputDirPath = filepath.Base(*targetDirPath)
	}

	if !*respectFileCase {
		*fileExtsCsv = strings.ToLower(*fileExtsCsv)
	}

	fileExts := strings.Split(*fileExtsCsv, ",")

	absTargetDirPath, err := filepath.Abs(*targetDirPath)
	if err != nil {
		log.Fatalf("failed to determine absolute path of target directory - %s", err.Error())
	}

	start := time.Now()

	d := newDoer(*numDecompilers)

	err = filepath.Walk(*targetDirPath,
		func(filePath string, info os.FileInfo, err error) error {
			if !*scanRecursively {
				if filepath.Dir(filePath) != absTargetDirPath {
					return filepath.SkipDir
				}
			}

			d.queue(func() error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				filename := info.Name()
				if !*respectFileCase {
					filename = strings.ToLower(filename)
				}
				for i := range fileExts {
					if strings.HasSuffix(filename, fileExts[i]) {
						extractedDirPath, err := extractNETFile(extractInfo{
							searchAbsPath:     absTargetDirPath,
							targetFileAbsPath: filePath,
							outputDirPath:     *outputDirPath,
						})
						if err != nil {
							if _, ok := err.(*ilspyError); ok {
								if *noIlspyErrors {
									return err
								}
								ioutil.WriteFile(filepath.Join(extractedDirPath, "extract-failure.log"), []byte(err.Error()), 0600)
								log.Printf("[warn] %s", err.Error())
								return nil
							}
							return fmt.Errorf("failed to extract '%s' - %s", filePath, err.Error())
						}

						log.Printf("extracted '%s' to '%s'", filePath, extractedDirPath)

						break
					}
				}
				return nil
			})

			return nil
		})
	if err != nil {
		log.Println(err)
	}

	err = d.wait()
	if err != nil {
		log.Fatalln(err.Error())
	}

	log.Printf("finished after %s", time.Since(start).String())
}

type extractInfo struct {
	searchAbsPath     string
	targetFileAbsPath string
	outputDirPath     string
}

func extractNETFile(info extractInfo) (string, error) {
	finalOutputDirPath := filepath.Join(info.outputDirPath,
		strings.TrimPrefix(info.targetFileAbsPath, info.searchAbsPath))

	err := os.MkdirAll(finalOutputDirPath, 0700)
	if err != nil {
		return "", fmt.Errorf("failed to create output subdirectory - %s", err)
	}

	raw, err := exec.Command("ilspycmd", info.targetFileAbsPath, "-p", "-o", finalOutputDirPath).CombinedOutput()
	if err != nil {
		return "", &ilspyError{
			err: fmt.Sprintf("failed to extract .net code from '%s' - %s - %s",
				info.targetFileAbsPath, err.Error(), raw),
		}
	}

	return finalOutputDirPath, nil
}

type ilspyError struct {
	err string
}

func (o ilspyError) Error() string {
	return o.err
}

func newDoer(numWorkers int) *doer {
	pool := make(chan int, numWorkers)
	for i := 0; i < numWorkers; i++ {
		pool <- i
	}

	return &doer{
		pool:   pool,
		failed: make(chan error, 1),
		dead:   make(chan struct{}),
		wg:     &sync.WaitGroup{},
	}
}

type doer struct {
	pool   chan int
	failed chan error
	dead   chan struct{}
	wg     *sync.WaitGroup
}

func (o *doer) queue(fn func() error) {
	o.wg.Add(1)
	go func() {
		select {
		case workerID := <-o.pool:
			go func() {
				err := fn()
				if err != nil {
					select {
					case o.failed <- err:
					default:
						close(o.dead)
					}
					o.wg.Done()
					return
				}
				o.wg.Done()
				o.pool <- workerID
			}()
		case <-o.dead:
			return
		}
	}()
}

func (o *doer) wait() error {
	o.wg.Wait()
	select {
	case err := <-o.failed:
		return err
	default:
		return nil
	}
}
