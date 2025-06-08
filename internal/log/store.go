package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

var (
	enc = binary.BigEndian // 레코드 크기와 인덱스 항목을 저장할때의 인코딩
)

const (
	lenWidth = 8
)

type store struct {
	*os.File
	mu   sync.Mutex
	buf  *bufio.Writer
	size uint64
}

// 데이터가 있는 파일로 스토어를 생성할 때 사용한다.(예를들어 서비스를 재시작할때)
func newStore(f *os.File) (*store, error) {
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}

	size := uint64(fi.Size())
	return &store{
		File: f,
		size: size,
		buf:  bufio.NewWriter(f),
	}, nil
}

func (s *store) Append(p []byte) (n uint64, pos uint64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pos = s.size // 데이터를 쓰기 시작한 위치

	// record 크기 write(구조화된 데이터를 바이트형식으로 인코딩하여 write)
	if err := binary.Write(s.buf, enc, uint64(len(p))); err != nil {
		return 0, 0, err
	}

	// record write(이미 완성된 []byte데이터 write)
	w, err := s.buf.Write(p)
	if err != nil {
		return 0, 0, err
	}

	w += lenWidth // record크기도 더함
	s.size += uint64(w)
	return uint64(w), pos, nil
}

func (s *store) Read(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 읽으려는 레코드가 아직 버퍼에 있을 때를 대비하여 버퍼의 내용을 flush해서 디스크에 작성(성능 최적화를 위함)
	if err := s.buf.Flush(); err != nil {
		return nil, err
	}

	// record 크기 read
	size := make([]byte, lenWidth)
	if _, err := s.File.ReadAt(size, int64(pos)); err != nil {
		return nil, err
	}

	// record read
	b := make([]byte, enc.Uint64(size))
	if _, err := s.File.ReadAt(b, int64(pos+lenWidth)); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *store) ReadAt(p []byte, off int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.buf.Flush(); err != nil {
		return 0, err
	}
	return s.File.ReadAt(p, off)
}

func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.buf.Flush(); err != nil {
		return err
	}
	return s.File.Close()
}
