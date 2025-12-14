# 实时聊天工具

基于UDP实现了简单的文本发送和接收功能 

界面使用[gioui](https://github.com/gioui/gio)搭建  

具体实现参考了[protonet](https://github.com/mearaj/protonet)项目

# 特性

不需要账号就能使用  

设置相同暗号的人可以一起聊天  

后续支持p2p通信，减少对服务器的依赖  

设计原则：极简主义，keep it simple and stupid

# 后续

1. 支持语音、电话、视频交流  
2. 支持p2p通信，相关库：[libp2p](https://github.com/libp2p/go-libp2p)  
3. 表情包，消息框支持图片边框
4. 一对一聊天、消息状态回执
5. 支持发送文件，图片，文档，压缩包等等
6. 跟DeepSeek类似的markdown文本
7. 支持收发邮件
8. bit torrent

# 想法

尽量使用自己写的代码，尽力重复造轮子，第三方库能不用就不用  

不重复造轮子？前提是有造轮子的能力，不能和不想是两码事  

首先解决能不能的问题，然后再解决想不想的问题  

开发语言选择go主要是想学习go相关技术  
