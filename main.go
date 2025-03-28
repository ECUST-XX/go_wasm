//go:build js
// +build js

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"syscall/js"
	"time"

	"github.com/disintegration/imaging"
)

// GOOS=js GOARCH=wasm go build -o ./assets/phash.wasm
func main() {
	fmt.Println("go wasm init...")

	js.Global().Set("phash", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return ""
		}

		imgConStr := args[0].String()
		imgCon, err := base64.StdEncoding.DecodeString(imgConStr)
		if err != nil {
			fmt.Println("decode err", imgConStr)
			return err.Error()
		}

		return phash(imgCon)
	}))

	// 异步代码思路依然是js回调，类比go会死锁
	js.Global().Set("wget", js.FuncOf(func(this js.Value, args []js.Value) any {
		dst := js.Global().Get("Uint8Array").New(100)
		ch := make(chan int)

		// 只能异步否则死锁
		go func(d js.Value) {
			res := wget()
			fmt.Println("copy len:", js.CopyBytesToJS(d, res))
			ch <- 1
		}(dst)

		// <-ch // 强行获取获取结果会导致死锁，只能sleep
		time.Sleep(time.Second)

		return dst
	}))

	// 测试tcp udp客户端
	// nc -lu 1234
	// nc -l 4567
	js.Global().Set("netudptcp", js.FuncOf(func(this js.Value, args []js.Value) any {
		// 无法发送udp包
		go netudp()
		// tcp链接被拒绝
		go nettcp()

		time.Sleep(time.Second)
		return nil
	}))

	<-make(chan int, 0)
}

func nettcp() {
	p := make([]byte, 2048)
	conn, err := net.Dial("tcp", "127.0.0.1:4567")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Fprintf(conn, "Hi tcp Server, How are you doing?")
	_, err = bufio.NewReader(conn).Read(p)
	if err == nil {
		fmt.Printf("tcp %s\n", p)
	} else {
		fmt.Println(err)
	}
	conn.Close()
}

func netudp() {
	p := make([]byte, 2048)
	conn, err := net.Dial("udp", "127.0.0.1:1234")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Fprintf(conn, "Hi UDP Server, How are you doing?")
	_, err = bufio.NewReader(conn).Read(p)
	if err == nil {
		fmt.Printf("udp %s\n", p)
	} else {
		fmt.Println(err)
	}
	conn.Close()
}

func wget() []byte {
	resp, err := http.Get("wasm_exec.html")
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer resp.Body.Close()

	cont, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Println(len(cont))
	return cont
}

func phash(imgCont []byte) string {
	// 1. 读取图片
	img, err := imaging.Decode(bytes.NewBuffer(imgCont))
	if err != nil {
		panic(err)
	}

	// 2. 计算感知哈希
	phashValue, err := ComputePHash(img)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Perceptual Hash: %016x\n", phashValue)

	return strconv.FormatInt(int64(phashValue), 16)
}

// ComputePHash 计算图像的感知哈希值
func ComputePHash(img image.Image) (uint64, error) {
	// 步骤1: 调整大小为32x32并转为灰度图
	resized := imaging.Resize(img, 32, 32, imaging.Lanczos)
	gray := imaging.Grayscale(resized)

	// 步骤2: 转换为二维灰度矩阵
	grayMatrix := imageToGrayMatrix(gray)

	// 步骤3: 计算DCT变换
	dctMatrix := applyDCT(grayMatrix)

	// 步骤4: 获取8x8低频部分 (排除第一个系数)
	lowFreq := make([]float64, 64)
	index := 0
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			if i == 0 && j == 0 {
				continue // 跳过DC系数
			}
			lowFreq[index] = dctMatrix[i][j]
			index++
		}
	}
	lowFreq = lowFreq[:63] // 取63个AC系数

	// 步骤5: 计算平均值
	mean := calculateMean(lowFreq)

	// 步骤6: 生成二进制哈希
	var hash uint64
	for i, val := range lowFreq {
		if val > mean {
			hash |= 1 << uint(63-i) // 64位哈希
		}
	}

	return hash, nil
}

// 辅助函数 ---------------------------------------------------

// 将图像转换为灰度矩阵
func imageToGrayMatrix(img image.Image) [][]float64 {
	bounds := img.Bounds()
	matrix := make([][]float64, bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := make([]float64, bounds.Dx())
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// 转换为YUV亮度值
			row[x] = 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
		}
		matrix[y] = row
	}
	return matrix
}

// 离散余弦变换 (DCT-II)
func applyDCT(matrix [][]float64) [][]float64 {
	size := len(matrix)
	dct := make([][]float64, size)

	// 一维DCT变换
	dct1D := func(input []float64) []float64 {
		output := make([]float64, len(input))
		n := float64(len(input))
		for k := range output {
			var sum float64
			for i, val := range input {
				sum += val * math.Cos(math.Pi*(float64(i)+0.5)*float64(k)/n)
			}
			if k == 0 {
				output[k] = sum * math.Sqrt(1.0/n)
			} else {
				output[k] = sum * math.Sqrt(2.0/n)
			}
		}
		return output
	}

	// 先对行变换
	for y := range matrix {
		dct[y] = dct1D(matrix[y])
	}

	// 对列变换
	for x := 0; x < size; x++ {
		column := make([]float64, size)
		for y := 0; y < size; y++ {
			column[y] = dct[y][x]
		}
		transformed := dct1D(column)
		for y := 0; y < size; y++ {
			dct[y][x] = transformed[y]
		}
	}

	return dct
}

// 计算平均值（排除DC系数）
func calculateMean(data []float64) float64 {
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}
