package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"runtime"
	"strings"
	"time"

	"strconv"

	"github.com/chaosblade-io/chaosblade/exec"
	"github.com/chaosblade-io/chaosblade/exec/os"
	"github.com/chaosblade-io/chaosblade/exec/os/bin"
	"github.com/chaosblade-io/chaosblade/util"
)

var (
	burnCpuStart, burnCpuStop, burnCpuNohup bool
	cpuCount                                int
	cpuList                                 string
	cpuProcessor                            string
)

const cpuProcessorFlag = "cpu-processor"

func main() {
	flag.BoolVar(&burnCpuStart, os.StartFlag, false, "burn cpu")
	flag.BoolVar(&burnCpuStop, os.StopFlag, false, "stop burn cpu")
	flag.StringVar(&cpuList, os.CpuListFlag, "", "CPUs in which to allow burning (1,3)")
	flag.BoolVar(&burnCpuNohup, os.NohupFlag, false, "nohup to run burn cpu")
	flag.IntVar(&cpuCount, os.CpuCountFlag, runtime.NumCPU(), "number of cpus")
	flag.StringVar(&cpuProcessor, cpuProcessorFlag, "0", "only used for identifying process of cpu burn")
	flag.Parse()

	if cpuCount <= 0 || cpuCount > runtime.NumCPU() {
		cpuCount = runtime.NumCPU()
	}

	if burnCpuStart {
		startBurnCpu()
	} else if burnCpuStop {
		stopBurnCpuFunc()
	} else if burnCpuNohup {
		burnCpu()
	} else {
		bin.PrintErrAndExit("less --start or --stop flag")
	}
}

func burnCpu() {
	runtime.GOMAXPROCS(cpuCount)

	for i := 0; i < cpuCount; i++ {
		go func() {
			for {
				for i := 0; i < 2147483647; i++ {
				}
				runtime.Gosched()
			}
		}()
	}
	select {} // wait forever
}

var channel = exec.NewLocalChannel()

var stopBurnCpuFunc = stopBurnCpu

var runBurnCpuFunc = runBurnCpu

var bindBurnCpuFunc = bindBurnCpu

var checkBurnCpuFunc = checkBurnCpu

// startBurnCpu by invoke burnCpuBin with --nohup flag
func startBurnCpu() {
	ctx := context.Background()
	if cpuList != "" {
		cpuCount = 1
		cores := strings.Split(cpuList, os.CommaDelimiter)
		for _, core := range cores {
			pid := runBurnCpuFunc(ctx, cpuCount, true, core)
			bindBurnCpuFunc(ctx, core, pid)
		}
	} else {
		runBurnCpuFunc(ctx, cpuCount, false, "")
	}
	checkBurnCpuFunc(ctx)
}

// runBurnCpu
func runBurnCpu(ctx context.Context, cpuCount int, pidNeeded bool, processor string) int {
	args := fmt.Sprintf(`%s --%s --%s %d`,
		path.Join(util.GetProgramPath(), os.BurnCpuCommand), os.NohupFlag, os.CpuCountFlag, cpuCount)

	if pidNeeded {
		args = fmt.Sprintf("%s --%s %s", args, cpuProcessorFlag, processor)
	}
	args = fmt.Sprintf(`%s > /dev/null 2>&1 &`, args)
	response := channel.Run(ctx, os.NohupCommand, args)
	if !response.Success {
		stopBurnCpuFunc()
		bin.PrintErrAndExit(response.Err)
	}
	if pidNeeded {
		// parse pid
		newCtx := context.WithValue(context.Background(), exec.ProcessKey, fmt.Sprintf("%s %s", cpuProcessorFlag, processor))
		pids, err := exec.GetPidsByProcessName(os.BurnCpuCommand, newCtx)
		if err != nil {
			stopBurnCpuFunc()
			bin.PrintErrAndExit(fmt.Sprintf("bind cpu core failed, cannot get the burning program pid, %v", err))
		}
		if len(pids) > 0 {
			// return the first one
			pid, err := strconv.Atoi(pids[0])
			if err != nil {
				stopBurnCpuFunc()
				bin.PrintErrAndExit(fmt.Sprintf("bind cpu core failed, get pid failed, pids: %v, err: %v", pids, err))
			}
			return pid
		}
	}
	return -1
}

// bindBurnCpu by taskset command
func bindBurnCpu(ctx context.Context, core string, pid int) {
	response := channel.Run(ctx, os.TasksetCommand, fmt.Sprintf("-cp %s %d", core, pid))
	if !response.Success {
		stopBurnCpuFunc()
		bin.PrintErrAndExit(response.Err)
	}
}

// checkBurnCpu
func checkBurnCpu(ctx context.Context) {
	time.Sleep(time.Second)
	// query process
	ctx = context.WithValue(ctx, exec.ProcessKey, os.NohupCommand)
	pids, _ := exec.GetPidsByProcessName(os.BurnCpuCommand, ctx)
	if pids == nil || len(pids) == 0 {
		bin.PrintErrAndExit(fmt.Sprintf("%s pid not found", os.BurnCpuCommand))
	}
}

// stopBurnCpu
func stopBurnCpu() {
	// add grep nohup
	ctx := context.WithValue(context.Background(), exec.ProcessKey, os.NohupCommand)
	pids, _ := exec.GetPidsByProcessName(os.BurnCpuCommand, ctx)
	if pids == nil || len(pids) == 0 {
		return
	}
	channel.Run(ctx, os.KillCommand, fmt.Sprintf(`-9 %s`, strings.Join(pids, " ")))
}
