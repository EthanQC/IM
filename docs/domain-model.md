## 领域模型图
本图展示系统核心领域对象及其关系

```mermaid
classDiagram
    %% 使用不同的样式区分不同类型的领域对象
    
    %% 聚合根(使用粗边框)
    class UserInfo:::root {
      +String id
      +String username 
      +String password
      +String phone
      +String avatar
      +Boolean disable
      +DateTime createTime
      +DateTime updateTime
      +List<UserContact> contacts
      +List<Session> sessions
    }

    class GroupInfo:::root {
      +String id
      +String name
      +String avatar 
      +String description
      +Boolean disable
      +String ownerId
      +List<String> adminIds
      +List<String> memberIds
      +DateTime createTime
      +DateTime updateTime
    }

    class Session:::root {
      +String id
      +String userId
      +String targetId
      +SessionType type
      +DateTime lastMessageTime
      +DateTime createTime
      +DateTime updateTime
    }

    %% 实体(使用普通边框)
    class Message:::entity {
      +String id
      +String senderId
      +String targetId
      +MessageType type
      +String content 
      +String fileUrl
      +Boolean isRead
      +DateTime sendTime
    }

    class UserContact:::entity {
      +String id
      +String userId
      +String contactId
      +String remark
      +DateTime createTime
    }

    class ContactApply:::entity {
      +String id
      +String userId
      +String targetId
      +String message
      +ApplyStatus status
      +DateTime createTime
    }

    class Notification:::entity {
      +String id
      +String userId
      +String title
      +String content
      +NotificationType type
      +Boolean isRead
      +DateTime createTime
    }

    class FileUpload:::entity {
      +String id
      +String userId
      +String fileName
      +String fileType
      +Long fileSize
      +String storePath
      +DateTime uploadTime
    }

    %% 值对象(使用虚线边框)
    class MessageContent:::value {
      +MessageType type
      +String text
      +FileAttachment attachment
    }

    class FileAttachment:::value {
      +String url
      +String name
      +String type
      +Long size
    }

    class AuthCode:::value {
      +String phone
      +String code
      +DateTime expireTime
      +Boolean isUsed
    }

    class Email:::value {
      +String address
      +Boolean verified
    }

    %% 领域服务(使用特殊边框)
    class ChatRoomService:::service {
      +broadcastMessage(message: Message)
      +getOnlineUsers(): List<UserInfo>
      +getUserRooms(userId: String): List<GroupInfo>
    }

    class WebRTCService:::service {
      +initCall(caller: UserInfo, callee: UserInfo)
      +endCall(sessionId: String)
    }

    class AuthService:::service {
      +generateAuthCode(phone: String)
      +verifyAuthCode(phone: String, code: String)
      +createToken(user: UserInfo)
      +validateToken(token: String)
    }

    class FileService:::service {
      +uploadFile(file: File): FileUpload
      +downloadFile(fileId: String)
      +deleteFile(fileId: String)
    }

    class NotificationService:::service {
      +sendNotification(notification: Notification)
      +markAsRead(notificationId: String)
      +getUserNotifications(userId: String)
    }

    %% 领域对象之间的关系(添加更清晰的描述)
    UserInfo "1" --o "0..*" UserContact : owns
    UserInfo "1" --o "0..*" Session : manages
    UserInfo "1" --> "0..*" Message : sends
    GroupInfo "1" --o "0..*" Message : contains
    GroupInfo "1" --> "0..*" UserInfo : has members
    Message "1" --* "1" MessageContent : contains
    MessageContent "1" --o "0..1" FileAttachment : may have
    UserInfo "1" --> "0..*" ContactApply : processes
    UserInfo "1" --o "0..*" Notification : receives
    UserInfo "1" --o "0..*" FileUpload : owns
    UserInfo "1" --* "0..1" Email : has
    Message "1" --o "0..1" FileUpload : contains
    AuthService "1" --> "0..*" AuthCode : manages
    NotificationService "1" --> "0..*" Notification : handles
    FileService "1" --> "0..*" FileUpload : manages

    %% 定义样式类
    classDef root fill:#9cf,stroke:#333,stroke-width:4px
    classDef entity fill:#bbf,stroke:#333,stroke-width:2px  
    classDef value fill:#f9c,stroke:#333,stroke-dasharray: 5 5
    classDef service fill:#ffd,stroke:#333,stroke-width:3px,stroke-dasharray: 10 5
```

### 说明

1. 聚合根:
- UserInfo: 用户信息聚合根
- GroupInfo: 群组信息聚合根 
- Session: 会话聚合根

2. 实体:
- Message: 消息实体
- UserContact: 联系人关系实体
- ContactApply: 好友申请实体
- Notification: 系统通知实体
- FileUpload: 文件上传记录实体

3. 值对象:
- MessageContent: 消息内容值对象
- FileAttachment: 文件附件值对象
- AuthCode: 手机验证码
- Email: 邮箱值对象

4. 领域服务:
- ChatRoomService: 负责消息广播和在线用户管理
- WebRTCService: 处理音视频通话
- AuthService: 处理登录认证相关
- FileService: 处理文件上传下载
- NotificationService: 处理系统通知

5. 关键业务规则:
- 用户可以发送多种类型消息(文本/文件/视频)
- 支持一对一聊天和群组聊天
- 联系人关系需要双方确认
- 群组有所有者和管理员角色
- 用户可以使用手机验证码或账号密码登录
- 支持文件的上传、下载和管理
- 系统消息通知(如好友申请、群组邀请等)
- 邮箱验证功能