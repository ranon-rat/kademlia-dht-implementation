package executioner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	executionerD "github.com/ranon-rat/kademlia-dht-implementation/src/domain/executioner"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/safe"
)

var runningProcesss = safe.NewMap[uint64, *executionerD.RunningProcess]()
var OutputChannels = safe.NewMap[uint64, *safe.SafeChan[string]]()
var deletingAll atomic.Bool

func StopProcess(id uint64) error {
	cmd, exists := runningProcesss.Get(id)
	if !exists {
		return nil
	}
	cmd.Cancel()
	stopCmd(cmd.Cmd)
	runningProcesss.Delete(id)
	output, exist := OutputChannels.Get(id)
	if !exist {
		return nil
	}
	output.Close()
	return nil
}

// this stops the process directly

// this starts the process, it changes tha paused status and then it runs it
func StartProcess(process *executionerD.StartProcess) error {

	_, exists := runningProcesss.Get(process.ID)
	if exists {
		return nil
	}

	Executioner(process)
	return nil

}

func Executioner(process *executionerD.StartProcess) {

	if rp, e := runningProcesss.Get(process.ID); e {
		rp.Cmd.Cancel()
		stopCmd(rp.Cmd)
		runningProcesss.Delete(process.ID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "bash", "-lc" /*"trap 'kill 0' SIGTERM; "+*/, process.Command)
	cmd.Dir = process.Dir
	cmd.Env = executableEnv
	// i have to configure the system process attributes to avoid any weird behaviour
	// this function is empty on windows but on linux and unix derived systems
	// it sets the children process group id, for it to be used. later on the stop cmd
	setSysProcAttr(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return
	}
	channel, exists := OutputChannels.Get(process.ID)
	if !exists {
		// since i am closing the channel, this allows me to avoid any weird behaviour
		channel = safe.NewSafeChan[string](executionerD.MAX_CHANNEL_BUFFER)
		OutputChannels.Set(process.ID, channel)
	}
	// this is constantly being written and readed, so it uses a rwmutex
	// it also helps to store the values and to also know and debug what happened before
	// since golang sends the log.Println to the stderr
	// i need to use this to avoid any weird shit later on
	// specially with the port binding
	lastErrOutput := executionerD.NewSecureErrContainer(executionerD.MAX_LINES_DEFAULT)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		OutReader(process.ID, stdout, channel, lastErrOutput)
		wg.Done()
	}()
	go func() {
		ErrReader(process.ID, stderr, channel, lastErrOutput)
		wg.Done()
	}()
	if err := cmd.Start(); err != nil {
		cancel()
		stopCmd(cmd)
		return
	}

	runningProcesss.Set(process.ID, &executionerD.RunningProcess{
		Cmd:    cmd,
		Cancel: cancel,
	})
	go func() {
		// in case of any interruption i am using this
		err := cmd.Wait()
		// it deletes the running process
		runningProcesss.Delete(process.ID)
		wg.Wait()
		lastErrStr := strings.ToLower(lastErrOutput.Content())
		fmt.Println(lastErrStr)
		if ctx.Err() == context.Canceled {
			//
			if err != nil {
				Send(channel, err.Error(), process.ID)
			}
			goto justClean
		}
		// i am managing the port binding just after this. I should have something to manage any weird behaviour
		if err != nil {

			Send(channel, err.Error(), process.ID)
			if strings.Contains(strings.ToLower(err.Error()), "bind") {
				log.Println("Port already in use, not restarting", process.ID)
				goto justClean
			}
		}
		if strings.Contains(lastErrStr, "bind") {
			log.Println("Port already in use, not restarting", process.ID)
			goto justClean
		}
		time.Sleep(1 * time.Second)
		return
		// the just clean part is to avoid getting inside an infite loop
	justClean:
		// bueno aqui no deberia de haber un problema pero
		channel.Close()
		OutputChannels.Delete(process.ID)
	}()
}
func OutReader(id uint64, buf io.ReadCloser, channel *safe.SafeChan[string], lastErr *executionerD.SecureErrContainer) {

	defer func() {
		if r := recover(); r != nil {
			output := fmt.Sprintf("Recovered from panic in ErrReader for %v: %v", id, r)
			lastErr.AppendValue(output)
			Send(channel, output, id)
		}
	}()
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		// aqui podriamos decir
		output := scanner.Text()
		//	fmt.Println(name, output)
		Send(channel, output, id)
		// here i am just notifying that i came from the stdout
		lastErr.FromStdout()

	}
	if err := scanner.Err(); err != nil {
		log.Printf("error reading stdout for %v: %v", id, err)
	}
}

func ErrReader(id uint64, buf io.ReadCloser, channel *safe.SafeChan[string], lastErr *executionerD.SecureErrContainer) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic in ErrReader for %v: %v\n", id, r)
		}
	}()
	scanner := bufio.NewScanner(buf)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		// i am just trying to check something i dont want to erase anything weird
		if lastErr.ComingFromStdOut() {
			lastErr.Clean()
		}
		output := scanner.Text()
		// so i am adding something new to the append value
		lastErr.AppendValue(output)
		Send(channel, output, id)

		// i just want to know what is happening this is for testing

	}
	if err := scanner.Err(); err != nil {
		log.Printf("error reading stderr for %v: %v", id, err)
	}
}
func Send(channel *safe.SafeChan[string], output string, processID uint64) {
	channel.AppendSend(output)
}

// i need to manage the way i am executing this
func StoppingAll(ctx context.Context) {
	deletingAll.Store(true)
	var wg sync.WaitGroup
	runningProcesss.Range(func(id uint64, rp *executionerD.RunningProcess) bool {
		wg.Add(1)
		defer wg.Done()
		rp.Cancel()
		stopCmd(rp.Cmd)
		return true
	})
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		log.Println("All processes stopped")
	case <-ctx.Done():
		log.Println("Shutdown interrupted by context cancel")
	}
}
