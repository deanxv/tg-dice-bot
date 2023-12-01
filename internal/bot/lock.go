package bot

import "sync"

// 在包级别定义一个映射，存储每个userID对应的互斥锁
var userLocks = make(map[int64]*sync.Mutex)
var userLocksMutex sync.Mutex

// getUserLock 根据userID获取对应的互斥锁，如果不存在则创建一个新的锁
func getUserLock(userID int64) *sync.Mutex {
	userLocksMutex.Lock()
	defer userLocksMutex.Unlock()

	if _, ok := userLocks[userID]; !ok {
		userLocks[userID] = &sync.Mutex{}
	}

	return userLocks[userID]
}
