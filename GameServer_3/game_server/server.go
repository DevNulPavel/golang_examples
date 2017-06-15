package gameserver

import (
	"log"
	"net"
)

type Server struct {
	listener       *net.Listener
	clients        map[int]*Client
	addChannel     chan *Client
	deleteChannel  chan *Client
	sendAllChannel chan bool
	exitChannel    chan bool
}

// Создание нового сервера
func NewServer() *Server {
	clients := make(map[int]*Client)
	addChannel := make(chan *Client)
	deleteChannel := make(chan *Client)
	sendAllChannel := make(chan bool)
	successChannel := make(chan bool)

	return &Server{
		nil,
		clients,
		addChannel,
		deleteChannel,
		sendAllChannel,
		successChannel,
	}
}

// Добавление клиента через очередь
func (server *Server) AddNewClient(c *Client) {
	server.addChannel <- c
}

// Удаление клиента через очередь
func (server *Server) DeleteClient(c *Client) {
	server.deleteChannel <- c
}

// Отправить всем сообщения через очередь
func (server *Server) SendAll() {
	server.sendAllChannel <- true
}

func (server *Server) ExitServer() {
	server.exitChannel <- true
}

func (server *Server) StartSyncListen() {
	go server.startAsyncSocketAcceptListener()
	server.mainQueueHandleFunction() // Блокируется выполнение на этом методе
}

// Отправка состояния конкретному клиенту
func (server *Server) sendStateToClient(c *Client) {
	// Создать состояние текущее
	clientStates := []ClienState{}
	for _, client := range server.clients {
		clientStates = append(clientStates, client.state)
	}

	// Отослать юзеру
	c.QueueSendAllStates(clientStates)
}

// Отправить всем сообщение
func (server *Server) sendAllNewState() {
	// Создать состояние текущее
	clientStates := make([]ClienState, 0, len(server.clients))
	for _, client := range server.clients {
		clientStates = append(clientStates, client.state)
	}

	// Отослать всем
	for _, c := range server.clients {
		c.QueueSendAllStates(clientStates)
	}
}

func (server *Server) addClientToMap(client *Client) {
	server.clients[client.id] = client
}

func (server *Server) deleteClientFromMap(client *Client) (bool) {
	// Даже если нету клиента в мапе - ничего страшного
    if _, exists := server.clients[client.id]; exists {
        delete(server.clients, client.id)
        return true
    }
    return false
}

// Работа с новыми соединением идет в отдельной горутине
func (server *Server) newAsyncServerConnectionHandler(c *net.Conn) {
	// Создание нового клиента
	client := NewClient(c, server)
	server.AddNewClient(client)  // Выставляем клиента в очередь на добавление
	client.StartSyncListenLoop() // Блокируется выполнение на данной функции, пока не выйдет клиент
	client.Close()
    log.Printf("Server goroutine closed for client %d\n", client.id)
}

// Обработка входящих подключений
func (server *Server) startAsyncSocketAcceptListener() {
	// Создание листенера
	createdListener, err := net.Listen("tcp", ":9999")
	if err != nil {
		log.Println("Server listener start error")
		server.ExitServer()
		return
	}
	defer createdListener.Close() // TODO: Может быть не нужно? уже есть в exitAsyncSocketListener

	// Сохраняем листенер для возможности закрытия
	server.listener = &createdListener

	for {
		// Для возможности выхода из цикла
		if server.listener == nil {
			return
		}

		// Ожидаем новое подключение
		c, err := (*server.listener).Accept()
		if err != nil {
            log.Print("Accept error") // Либо наш лиснер закрылся и надо будет выйти из цикла
			continue
		}

		// Раз появилось новое соединение - запускаем его в работу с отдельной горутине
		go server.newAsyncServerConnectionHandler(&c)
	}
}

// Выход из обработчика событий
func (server *Server) exitAsyncSocketListener() {
	if server.listener != nil {
		(*server.listener).Close()
		server.listener = nil
	}
}

// Основная функция прослушивания
func (server *Server) mainQueueHandleFunction() {
	// Обработка каналов в главной горутине
	for {
		select {
		// Добавление нового юзера
		case c := <-server.addChannel:
            log.Printf("Client %d added\n", c.id)
			server.addClientToMap(c)
			c.QueueSendCurrentClientState() // После добавления на сервере - отправляем клиенту состояние
			server.sendAllNewState()

		// Удаление клиента
		case c := <-server.deleteChannel:
			deleted := server.deleteClientFromMap(c)
            if deleted {
                log.Printf("Client %d deleted\n", c.id)
                server.sendAllNewState()
            }

		// Отправка сообщения всем клиентам
		case <-server.sendAllChannel:
			// Вызываем отправку сообщений всем клиентам
			server.sendAllNewState()

		// Завершение работы
		case <-server.exitChannel:
			server.exitAsyncSocketListener()
			return
		}
	}
}
