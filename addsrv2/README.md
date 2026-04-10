
# 实现了在go-kit框架下的grpc通信

（1）实现了一个分层的编码格式
（2）其中service是用来进行通信的结构体，用于通信后真正的代码逻辑执行
（3）endpoint是这是纯粹的业务逻辑入口。它不关心你是 gRPC 还是 HTTP，只负责处理请求并返回结果。
（4）transport将业务逻辑（Service）翻译成 gRPC 协议能够理解的语言，并拼装成一个完整的 gRPC 服务器
（5）main函数通过go group来设定一个协程组，可以将错误进行返回，交由主程序处理

整个流程就是：Transport → Endpoint → Service，然后原路返回。

例如：
当一个 Sum(A: 10, B: 2) 的请求通过网络打过来时，发生了以下步骤：
(1) Transport (入口) —— “翻译员”动作:grpctransport.NewServer 接收到 gRPC 原始消息。关键执行：它会调用你写的 decodeGRPCSumRequest。结果：把 Protobuf 格式的数据转成了 Go-kit 内部定义的 SumRequest 结构体。
(2) Endpoint (中间层) —— “办事处”动作：数据进入 MakeSumEndpoint 返回的匿名函数。作用：这里是处理“非业务逻辑”的最佳位置。比如：日志记录、限流、熔断、权限验证（Middleware）。传递：如果一切正常，它把 SumRequest 传给底层的 Service。
(3) Service (核心) —— “厨师”动作：执行你真正写的 AddService.Sum() 逻辑。结果：算出 $10 + 2 = 12$，返回一个 (int, error)。

## 实现了中间件
（1）在transport层实现中间件：需要在transport调用HTTPServer的时候注册我的调用我定义的中间件；
（2）在应用层实现中间件：不同的是通过定义一个新的结构体实现可插拔的形式，在调用相应的函数的时候定义自己的日志信息，然后在程序一开始的时候把我的新结构体进行注册；
（3）实现了rateLimit中间件：中间件的格式都是固定的，三层函数：第一层是配置层，初始化限流器，是go-kit的标准化中间件模板；第二层是连接层，接收下一个要执行的节点，返回包装好的新的EndPoint;第三层是执行层，就是需要执行的代码
(4)实现了metric的中间件：用来记录代码的调用信息，给代码加上一个“仪表盘”,其中lvs := []string{"method", "sum", "error", fmt.Sprint(err != nil)}是做分类，不同的运行结果会分到不同的槽位里面，方便后续的查看。

## 实现了调用其他服务端的中间件
在Concat里面调用其他服务的Trim函数去处理输入中存在的空字符，然后再进行拼装，不同的是这个是作为一个客户端去调用GRPC服务，因此在encode和decode上面与之前的并不一样，如需要验证，需要自己写一个Trim服务端启动，代码在https://www.liwenzhou.com/posts/Go/go-kit-tutorial-05/

## 实现了通过consul实现服务发现和负载均衡
代替手动建立客户端的方式，通过consul去分配地址。