package client

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func BenchmarkZeroCopyForward(b *testing.B) {
	b.Run("io.Copy", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			pr, pw := io.Pipe()
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				defer pw.Close()
				buf := make([]byte, 32*1024)
				for j := 0; j < 100; j++ {
					pw.Write(buf)
				}
			}()

			go func() {
				defer wg.Done()
				io.Copy(io.Discard, pr)
			}()

			wg.Wait()
		}
	})

	b.Run("io.CopyBuffer", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			pr, pw := io.Pipe()
			buf := make([]byte, 128*1024)
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				defer pw.Close()
				buf2 := make([]byte, 32*1024)
				for j := 0; j < 100; j++ {
					pw.Write(buf2)
				}
			}()

			go func() {
				defer wg.Done()
				io.CopyBuffer(io.Discard, pr, buf)
			}()

			wg.Wait()
		}
	})
}

func BenchmarkTCPForward(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		defer conn.Close()
		time.Sleep(time.Second)
	}()

	b.Run("io.Copy", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			conn, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				b.Fatal(err)
			}

			io.Copy(io.Discard, conn)
			conn.Close()
		}
	})
}

func BenchmarkBidirectionalForward(b *testing.B) {
	b.Run("ZeroCopy", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			pr1, pw1 := io.Pipe()
			pr2, pw2 := io.Pipe()
			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				io.Copy(pw2, pr1)
			}()

			go func() {
				defer wg.Done()
				io.Copy(pw1, pr2)
			}()

			time.Sleep(time.Microsecond)

			pw1.Close()
			pw2.Close()
			wg.Wait()
		}
	})
}

type dummyReader struct{}

func (d *dummyReader) Read(p []byte) (int, error) {
	return len(p), nil
}

func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("ZeroCopy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r := &dummyReader{}
			io.Copy(io.Discard, r)
		}
	})
}
