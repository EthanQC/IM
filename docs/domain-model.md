## 领域模型图
本图展示系统核心领域对象及其关系

```mermaid
classDiagram
    %% 聚合根
    class UserInfo {
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

    class GroupInfo {
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

    class Session {
      +String id
      +String userId
      +String targetId
      +SessionType type
      +DateTime lastMessageTime
      +DateTime createTime
      +DateTime updateTime
    }

    %% 实体
    class Message {
      +String id
      +String senderId
      +String targetId
      +MessageType type
      +String content
      +String fileUrl
      +Boolean isRead
      +DateTime sendTime
    }

    class UserContact {
      +String id
      +String userId
      +String contactId
      +String remark
      +DateTime createTime
    }

    class ContactApply {
      +String id
      +String userId
      +String targetId
      +String message
      +ApplyStatus status
      +DateTime createTime
    }

    %% 值对象
    class MessageContent {
      +MessageType type
      +String text
      +FileAttachment attachment
    }

    class FileAttachment {
      +String url
      +String name
      +String type
      +Long size
    }

    %% 领域服务
    class ChatRoomService {
      +broadcastMessage(message: Message)
      +getOnlineUsers(): List<UserInfo>
      +getUserRooms(userId: String): List<GroupInfo>
    }

    class WebRTCService {
      +initCall(caller: UserInfo, callee: UserInfo)
      +endCall(sessionId: String)
    }

    %% 关系
    UserInfo "1" -- "0..*" UserContact : has
    UserInfo "1" -- "0..*" Session : owns
    UserInfo "1" -- "0..*" Message : sends
    GroupInfo "1" -- "0..*" Message : contains
    GroupInfo "1" -- "0..*" UserInfo : includes
    Message "1" -- "1" MessageContent : consists of
    MessageContent "0..1" -- "0..1" FileAttachment : may have
    UserInfo "1" -- "0..*" ContactApply : manages
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

3. 值对象:
- MessageContent: 消息内容值对象
- FileAttachment: 文件附件值对象

4. 领域服务:
- ChatRoomService: 负责消息广播和在线用户管理
- WebRTCService: 处理音视频通话

5. 关键业务规则:
- 用户可以发送多种类型消息(文本/文件/视频)
- 支持一对一聊天和群组聊天
- 联系人关系需要双方确认
- 群组有所有者和管理员角色