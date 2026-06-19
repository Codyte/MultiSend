package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lab/multinet/internal/proto"
	"lab/multinet/internal/split"
)

type progressReader struct {
	r       io.Reader
	counter *int64
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		atomic.AddInt64(p.counter, int64(n))
	}
	return n, err
}

func main() {
	filePath := flag.String("file", "", "file to send")
	server := flag.String("server", "", "server host:port")
	cableIP := flag.String("cable-ip", "", "local cable IP")
	wifiIP := flag.String("wifi-ip", "", "local wifi IP")
	ratio := flag.String("ratio", "2:1", "ratio left:right")
	flag.Parse()

	if *filePath == "" || *server == "" || *cableIP == "" || *wifiIP == "" {
		exitf("missing required flags")
	}

	leftW, rightW, err := parseRatio(*ratio)
	if err != nil {
		exitf("invalid ratio: %v", err)
	}

	f, err := os.Open(*filePath)
	if err != nil {
		exitf("open file: %v", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		exitf("stat file: %v", err)
	}

	aSize, bSize, err := split.ComputeParts(st.Size(), leftW, rightW)
	if err != nil {
		exitf("compute parts: %v", err)
	}

	base := filepath.Base(*filePath)

	var sentCable int64
	var sentWifi int64

	part1 := &progressReader{
		r:       io.NewSectionReader(f, 0, aSize),
		counter: &sentCable,
	}

	part2 := &progressReader{
		r:       io.NewSectionReader(f, aSize, bSize),
		counter: &sentWifi,
	}

	fmt.Println("multisend split-only")
	fmt.Printf("file:   %s\n", *filePath)
	fmt.Printf("server: %s\n", *server)
	fmt.Printf("cable:  %s -> part1 (%s)\n", *cableIP, humanBytes(aSize))
	fmt.Printf("wifi:   %s -> part2 (%s)\n", *wifiIP, humanBytes(bSize))
	fmt.Println()

	done := make(chan struct{})

	go renderProgress(done, &sentCable, &sentWifi, aSize, bSize)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := sendPart(
			ctx,
			*server,
			*cableIP,
			proto.Header{
				FileName:   base,
				PartIndex:  1,
				TotalParts: 2,
				PartSize:   aSize,
			},
			part1,
		)
		if err != nil {
			cancel()
		}
		errCh <- err
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := sendPart(
			ctx,
			*server,
			*wifiIP,
			proto.Header{
				FileName:   base,
				PartIndex:  2,
				TotalParts: 2,
				PartSize:   bSize,
			},
			part2,
		)
		if err != nil {
			cancel()
		}
		errCh <- err
	}()

	wg.Wait()
	close(done)
	fmt.Println()

	close(errCh)
	for e := range errCh {
		if e != nil {
			exitf("send failed: %v", e)
		}
	}

	fmt.Println("send complete")
}

func sendPart(ctx context.Context, server, localIP string, h proto.Header, r io.Reader) error {
	parsedIP := net.ParseIP(localIP)
	if parsedIP == nil {
		return fmt.Errorf("invalid local IP: %s", localIP)
	}

	d := net.Dialer{
		LocalAddr: &net.TCPAddr{IP: parsedIP},
		Timeout:   15 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	conn, err := d.DialContext(ctx, "tcp", server)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		conn.SetDeadline(time.Now())
	}()

	if err := proto.WriteHeader(conn, h); err != nil {
		return err
	}

	_, err = io.Copy(conn, r)
	return err
}

func renderProgress(done <-chan struct{}, cable, wifi *int64, cableTotal, wifiTotal int64) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	start := time.Now()
	var lastCable int64
	var lastWifi int64
	lastTick := start

	printOnce := func(final bool) {
		now := time.Now()
		elapsedTick := now.Sub(lastTick).Seconds()
		if elapsedTick <= 0 {
			elapsedTick = 1
		}

		c := atomic.LoadInt64(cable)
		w := atomic.LoadInt64(wifi)

		cableSpeed := float64(c-lastCable) * 8 / elapsedTick / 1_000_000
		wifiSpeed := float64(w-lastWifi) * 8 / elapsedTick / 1_000_000
		totalSpeed := cableSpeed + wifiSpeed

		totalDone := c + w
		totalSize := cableTotal + wifiTotal

		elapsedTotal := now.Sub(start).Seconds()
		if elapsedTotal <= 0 {
			elapsedTotal = 1
		}
		avgSpeed := float64(totalDone) * 8 / elapsedTotal / 1_000_000

		if final {
			cableSpeed = 0
			wifiSpeed = 0
			totalSpeed = 0
		}

		clearScreen()
		fmt.Println("transfer progress")
		fmt.Println()

		fmt.Printf("cable %s %6.2f%%  %10s / %-10s  %8.2f Mbit/s\n",
			bar(c, cableTotal, 32),
			percent(c, cableTotal),
			humanBytes(c),
			humanBytes(cableTotal),
			cableSpeed,
		)

		fmt.Printf("wifi  %s %6.2f%%  %10s / %-10s  %8.2f Mbit/s\n",
			bar(w, wifiTotal, 32),
			percent(w, wifiTotal),
			humanBytes(w),
			humanBytes(wifiTotal),
			wifiSpeed,
		)

		fmt.Println()

		fmt.Printf("total %s %6.2f%%  %10s / %-10s  %8.2f Mbit/s now | %8.2f Mbit/s avg\n",
			bar(totalDone, totalSize, 32),
			percent(totalDone, totalSize),
			humanBytes(totalDone),
			humanBytes(totalSize),
			totalSpeed,
			avgSpeed,
		)

		lastCable = c
		lastWifi = w
		lastTick = now
	}

	printOnce(false)

	for {
		select {
		case <-ticker.C:
			printOnce(false)
		case <-done:
			printOnce(true)
			return
		}
	}
}

func bar(done, total int64, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("-", width) + "]"
	}

	filled := int((done * int64(width)) / total)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func percent(done, total int64) float64 {
	if total <= 0 {
		return 100
	}
	return float64(done) * 100 / float64(total)
}

func humanBytes(n int64) string {
	const unit = 1024

	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	div := int64(unit)
	exp := 0
	for n >= div*unit && exp < 4 {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.2f %s", float64(n)/float64(div), units[exp])
}

func parseRatio(s string) (int64, int64, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected A:B")
	}

	var a, b int64
	_, err := fmt.Sscanf(s, "%d:%d", &a, &b)
	if err != nil {
		return 0, 0, err
	}

	if a <= 0 || b <= 0 {
		return 0, 0, fmt.Errorf("weights must be > 0")
	}

	return a, b, nil
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func clearScreen() {
	if runtime.GOOS == "windows" {
		fmt.Print("\033[H")
		return
	}
	fmt.Print("\033[H")
}
