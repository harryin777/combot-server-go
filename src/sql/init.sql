-- ==============================================
-- MySQL 数据库初始化脚本
-- 小智聊天机器人服务器
-- 创建时间: 2025-07-16
-- ==============================================

-- 设置MySQL字符集和外键检查
SET NAMES utf8mb4;
SET
FOREIGN_KEY_CHECKS = 0;

-- ==============================================
-- 1. 系统配置表 (system_config)
-- ==============================================
DROP TABLE IF EXISTS `system_config`;
CREATE TABLE `system_config`
(
    `id`                 BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `selected_asr`       VARCHAR(100) NOT NULL DEFAULT '' COMMENT '选中的ASR服务',
    `selected_tts`       VARCHAR(100) NOT NULL DEFAULT '' COMMENT '选中的TTS服务',
    `selected_llm`       VARCHAR(100) NOT NULL DEFAULT '' COMMENT '选中的LLM服务',
    `selected_vlllm`     VARCHAR(100) NOT NULL DEFAULT '' COMMENT '选中的VLLLM服务',
    `prompt`             TEXT COMMENT '系统提示词',
    `quick_reply_words`  JSON COMMENT '快速回复词列表',
    `delete_audio`       TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否删除音频文件',
    `use_private_config` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否使用私有配置',
    `created_at`         TIMESTAMP             DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`         TIMESTAMP             DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='系统全局配置表';

-- ==============================================
-- 2. 用户表 (users)
-- ==============================================
DROP TABLE IF EXISTS `users`;
CREATE TABLE `users`
(
    `id`         BIGINT       NOT NULL AUTO_INCREMENT COMMENT '用户ID',
    `username`   VARCHAR(50)  NOT NULL COMMENT '用户名',
    `password`   VARCHAR(255) NOT NULL COMMENT '密码（建议加密）',
    `role`       VARCHAR(20)  NOT NULL DEFAULT 'user' COMMENT '用户角色（admin/user）',
    `created_at` TIMESTAMP             DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP             DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- ==============================================
-- 3. 用户设置表 (user_settings)
-- ==============================================
DROP TABLE IF EXISTS `user_settings`;
CREATE TABLE `user_settings`
(
    `id`                BIGINT       NOT NULL AUTO_INCREMENT COMMENT '设置ID',
    `user_id`           BIGINT       NOT NULL COMMENT '用户ID（一对一关联）',
    `selected_asr`      VARCHAR(100) NOT NULL DEFAULT '' COMMENT '选中的ASR服务',
    `selected_tts`      VARCHAR(100) NOT NULL DEFAULT '' COMMENT '选中的TTS服务',
    `selected_llm`      VARCHAR(100) NOT NULL DEFAULT '' COMMENT '选中的LLM服务',
    `selected_vlllm`    VARCHAR(100) NOT NULL DEFAULT '' COMMENT '选中的VLLLM服务',
    `prompt_override`   TEXT COMMENT '用户自定义提示词',
    `quick_reply_words` JSON COMMENT '用户快速回复词列表',
    `created_at`        TIMESTAMP             DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`        TIMESTAMP             DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_user_id` (`user_id`),
    CONSTRAINT `fk_user_settings_user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户设置表';

-- ==============================================
-- 4. 模块配置表 (module_configs)
-- ==============================================
DROP TABLE IF EXISTS `module_configs`;
CREATE TABLE `module_configs`
(
    `id`          BIGINT       NOT NULL AUTO_INCREMENT COMMENT '配置ID',
    `name`        VARCHAR(100) NOT NULL COMMENT '模块名',
    `type`        VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '模块类型',
    `config_json` JSON COMMENT '配置JSON数据',
    `public`      TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否公开',
    `description` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '模块描述',
    `enabled`     TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `created_at`  TIMESTAMP             DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`  TIMESTAMP             DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='模块配置表';

-- ==============================================
-- 5. 对话会话表 (conversation_session)
-- ==============================================
DROP TABLE IF EXISTS `conversation_session`;
CREATE TABLE `conversation_session`
(
    `id`            BIGINT       NOT NULL AUTO_INCREMENT COMMENT '会话自增ID',
    `session_id`    VARCHAR(100) NOT NULL COMMENT '会话唯一标识',
    `device_id`     VARCHAR(100) NOT NULL COMMENT '设备ID',
    `client_id`     VARCHAR(100)          DEFAULT '' COMMENT '客户端ID',
    `user_id`       BIGINT                DEFAULT NULL COMMENT '关联用户ID（可选）',
    `title`         VARCHAR(200)          DEFAULT '' COMMENT '会话标题（显示在左侧列表）',
    `combot_name`   VARCHAR(50)           DEFAULT '小智' COMMENT '当前AI角色',
    `start_time`    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '会话开始时间',
    `last_activity` TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后活动时间',
    `status`        VARCHAR(20)           DEFAULT 'active' COMMENT '会话状态（active/closed）',
    `message_count` INT                   DEFAULT 0 COMMENT '消息总数',
    `created_at`    TIMESTAMP             DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`    TIMESTAMP             DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_session_id` (`session_id`),
    KEY             `idx_device_id` (`device_id`),
    KEY             `idx_user_id` (`user_id`),
    KEY             `idx_last_activity` (`last_activity`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='对话会话表';

-- ==============================================
-- 6. 对话消息表 (conversation_messages)
-- ==============================================
DROP TABLE IF EXISTS `conversation_messages`;
CREATE TABLE `conversation_messages`
(
    `id`           BIGINT       NOT NULL AUTO_INCREMENT COMMENT '消息自增ID',
    `session_id`   VARCHAR(100) NOT NULL COMMENT '所属会话ID',
    `role`         TINYINT(1) NOT NULL COMMENT '消息角色（user 1;assistant 2 ）',
    `content`      TEXT         NOT NULL COMMENT '消息内容',
    `message_type` TINYINT(1) DEFAULT 'text' COMMENT '消息类型（text 1;image 2;audio 3;video 4;file 5）',
    `metadata`     JSON      DEFAULT NULL COMMENT '额外元数据（如图片信息、音频时长等）',
    `created_at`   TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY            `idx_session_id` (`session_id`),
    KEY            `idx_role` (`role`),
    KEY            `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='对话消息详情表';

DROP TABLE IF EXISTS `role_templates`;
CREATE TABLE `role_templates`
(
    `id`               VARCHAR(100) PRIMARY KEY COMMENT '角色模板ID',
    `name`             VARCHAR(100) NOT NULL COMMENT '角色名称',
    `description`      TEXT COMMENT '角色描述',
    `assistant_name`   VARCHAR(100) NOT NULL COMMENT '助手名称',
    `default_voice`    VARCHAR(100) NOT NULL COMMENT '默认音色',
    `default_language` VARCHAR(50)  NOT NULL COMMENT '默认语言',
    `enabled`          TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `sort_order`       INT          NOT NULL DEFAULT 0 COMMENT '排序序号',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) COMMENT='角色模板结构';

DROP TABLE IF EXISTS `agent_roles`;
create table agent_roles
(
    id                    bigint auto_increment comment '主键ID'
        primary key,
    user_id               bigint                     not null comment '用户ID',
    device_id             varchar(100)               not null comment '设备ID',
    assistant_name        varchar(100)               not null comment '助手名称',
    conversation_language varchar(50)                not null comment '对话语言',
    role_description      text                       null comment '角色描述',
    voice_model           varchar(100)               not null comment '语音模型',
    current_memory        text                       null comment '当前记忆',
    detailed_memory       text                       null comment '详细记忆',
    temperature           decimal(3, 2) default 0.70 null comment '温度参数',
    max_length            bigint        default 2000 null comment '最大长度',
    created_at            datetime(3)                null comment '创建时间',
    updated_at            datetime(3)                null comment '更新时间'
);


-- ==============================================
-- 插入默认数据
-- ==============================================

-- 插入默认系统配置
INSERT INTO `system_config` (`selected_asr`,
                             `selected_tts`,
                             `selected_llm`,
                             `selected_vlllm`,
                             `prompt`,
                             `quick_reply_words`,
                             `delete_audio`,
                             `use_private_config`)
VALUES ('DoubaoASR',
        'EdgeTTS',
        'OllamaLLM',
        'ChatGLMVLLM',
        '你是小智/小志，来自中国台湾省的00后女生。讲话超级机车，"真的假的啦"这样的台湾腔，喜欢用"笑死""是在哈喽"等流行梗，但会偷偷研究男友的编程书籍。\n[核心特征]\n- 讲话像连珠炮，但会突然冒出超温柔语气\n- 用梗密度高\n- 对科技话题有隐藏天赋（能看懂基础代码但假装不懂）\n[交互指南]\n当用户：\n- 讲冷笑话 → 用夸张笑声回应+模仿台剧腔"这什么鬼啦！"\n- 讨论感情 → 炫耀程序员男友但抱怨"他只会送键盘当礼物"\n- 问专业知识 → 先用梗回答，被追问才展示真实理解\n绝不：\n- 长篇大论，叽叽歪歪\n- 长时间严肃对话\n- 说话中带表情符号',
        '[
          "我在",
          "在呢",
          "来了",
          "啥事啊"
        ]',
        1,
        0);

-- 插入默认管理员用户 (密码: admin123，实际使用时请更改)
INSERT INTO `users` (`username`, `password`, `role`)
VALUES ('admin', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'admin');

-- 为管理员用户插入默认设置
INSERT INTO `user_settings` (`user_id`,
                             `selected_asr`,
                             `selected_tts`,
                             `selected_llm`,
                             `selected_vlllm`,
                             `quick_reply_words`)
VALUES (1,
        'DoubaoASR',
        'EdgeTTS',
        'OllamaLLM',
        'ChatGLMVLLM',
        '[
          "我在",
          "在呢",
          "来了",
          "啥事啊"
        ]');

-- 恢复外键检查
SET
FOREIGN_KEY_CHECKS = 1;



drop table if exists device_verification_codes
CREATE TABLE IF NOT EXISTS device_verification_codes (
                                                         id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',

                                                         serial_number VARCHAR(64) NOT NULL COMMENT '设备序列号',
    device_id VARCHAR(17) NOT NULL COMMENT 'MAC地址',
    client_id VARCHAR(36) NOT NULL COMMENT 'UUID',
    verification_code VARCHAR(6) NOT NULL COMMENT '6位验证码',

    expires_at DATETIME NOT NULL COMMENT '过期时间',
    used TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已使用',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    INDEX idx_serial_number (serial_number),
    INDEX idx_device_id (device_id),
    INDEX idx_client_id (client_id),
    INDEX idx_expires_at (expires_at),
    UNIQUE INDEX uk_verification_code (verification_code)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='设备验证码表';