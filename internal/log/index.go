package log

import (
	"io"
	"os"

	"github.com/tysonmote/gommap"
)

var (
	offWidth uint64 = 4
	posWidth uint64 = 8
	entWidth        = offWidth + posWidth
)

type index struct {
	file *os.File
	mmap gommap.MMap
	size uint64 // index크기, 다음 항목을 추가할 위치를 의미
}

func newIndex(f *os.File, c Config) (*index, error) {
	idx := &index{
		file: f,
	}

	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}

	idx.size = uint64(fi.Size()) // 파일의 현재 크기를 저장(인덱스 파일의 데이터양 추적을 위해)
	if err = os.Truncate(        // mmap을 위해 Truncate
		f.Name(), int64(c.Segment.MaxIndexBytes),
	); err != nil {
		return nil, err
	}

	// 파일을 메모리에 직접 매핑하여, 메모리에서 읽고쓰는 것만으로도 파일내용 조작가능
	if idx.mmap, err = gommap.Map(
		idx.file.Fd(),
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED, //메모리에서 변경한 내용이 디스크에도 반영
	); err != nil {
		return nil, err
	}

	return idx, nil
}

func (i *index) Close() error {
	if err := i.mmap.Sync(gommap.MS_SYNC); err != nil { // mmap 동기화(디스크로 flush)
		return err
	}

	if err := i.file.Sync(); err != nil { // 실제 파일 동기화(file.write로 썼을 수도 있으므로)
		return err
	}

	if err := i.file.Truncate(int64(i.size)); err != nil { // 최대로 늘린 파일을 실제 파일 크기만큼 Truncate
		return err
	}

	return i.file.Close()
}

func (i *index) Read(in int64) (out uint32, pos uint64, err error) {
	if i.size == 0 {
		return 0, 0, io.EOF
	}

	if in == -1 { // 마지막 entry
		out = uint32((i.size / entWidth) - 1)
	} else {
		out = uint32(in)
	}

	pos = uint64(out) * entWidth
	if i.size < pos+entWidth { // 읽으려는 entry위치
		return 0, 0, io.EOF
	}

	out = enc.Uint32(i.mmap[pos : pos+offWidth])
	pos = enc.Uint64(i.mmap[pos+offWidth : pos+entWidth]) //실제 메시지
	return out, pos, nil
}

func (i *index) Write(off uint32, pos uint64) error {
	if uint64(len(i.mmap)) < i.size+entWidth {
		return io.EOF
	}

	enc.PutUint32(i.mmap[i.size:i.size+offWidth], off)
	enc.PutUint64(i.mmap[i.size+offWidth:i.size+entWidth], pos)
	i.size += uint64(entWidth)

	return nil
}

func (i *index) Name() string {
	return i.file.Name()
}
