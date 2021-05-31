package main

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var ErrPoolClosed = errors.New("pool is shutdown")

type Pool struct {
	// m是一个互斥锁，用来保证在多个goroutine访问资源时，池内的值是安全的。
	mutex sync.Mutex
	// res字段是一个有缓冲的通道，用来保存共享的资源, 实现了这个io.Closer接口的类型都可以作为资源，交给我们的资源池管理。
	res chan io.Closer
	// factory这个是一个函数类型，它的作用就是当需要一个新的资源时，可以通过这个函数创建，也就是说它是生成新资源的，至于如何生成、生成什么资源，是由使用者决定的，所以这也是这个资源池灵活的设计的地方。
	factory func()(io.Closer, error)
	// closed字段表示资源池是否被关闭，如果被关闭的话，再访问是会有错误的。
	closed bool
}

// create a pool
func NewPool(fn func()(io.Closer, error), size uint) (*Pool, error) {
	if size <= 0 {
		return nil, errors.New("size must be greater than 0")
	}
	return &Pool{
		factory: fn,
		res: make(chan io.Closer, size),
	}, nil
}

// close pool, releases resources
func (p *Pool) Close() error{
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.closed {
		return ErrPoolClosed
	}
	p.closed = true
	// close channel of pool
	close(p.res)
	// close resource in pool
	for r := range p.res {
		return r.Close()
	}
	return nil
}

// acquire resource from pool
func (p *Pool) Acquire() (io.Closer, error) {
	select {
	case r, ok := <- p.res:
		fmt.Println("Acquire: shared resource")
		if !ok {
			return nil, ErrPoolClosed
		}
		return r, nil
	default:
		fmt.Println("Acquire: new resource")
		return p.factory()
	}

}

func (p *Pool) Release(r io.Closer) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	// 使用了互斥锁，保证closed标志的安全，而且这个互斥锁还有一个好处，就是不会往一个已经关闭的通道发送资源。
	if p.closed {
		return r.Close()
	}
	select {
	case p.res <- r:
		fmt.Println("release resource to pool")
		return nil
	default:
		fmt.Println("the pool is full, close resource")
		return r.Close()
	}
}


// test pool

type dbConnection struct {
	Id int32
}

func (db *dbConnection) Close() error {
	fmt.Printf("close connection: %d\n", db.Id)
	return nil
}

var idCounter int32

func createConnection() (io.Closer, error) {
	id := atomic.AddInt32(&idCounter, 1)
	return &dbConnection{Id: id}, nil
}

func dbQuery(query int, pool *Pool)  {
	conn, err := pool.Acquire()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer pool.Release(conn)
	// mock query
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
	fmt.Printf("第%d个查询，使用的是ID为%d的数据库连接\n", query, conn.(*dbConnection).Id)
}

const (
	maxGoroutine = 5
	poolSize = 2
)

func main()  {
	var wg sync.WaitGroup
	wg.Add(maxGoroutine)
	pool, err := NewPool(createConnection, poolSize)
	if err != nil {
		fmt.Println(err)
		return
	}
	for query := 0; query < maxGoroutine; query++ {
		go func(q int) {
			defer wg.Done()
			dbQuery(q, pool)
		}(query)
	}

	wg.Wait()
	pool.Close()
}

/*
资源对象池的使用比较频繁，因为我们想把一些对象缓存起来，以便使用，
这样就会比较高效，而且不会经常调用GC，为此Go为我们提供了原生的资源池管理，防止我们重复造轮子，这就是sync.Pool
 */


