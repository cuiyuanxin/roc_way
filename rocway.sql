/*
 Navicat Premium Dump SQL

 Source Server         : 本地数据库
 Source Server Type    : MySQL
 Source Server Version : 80012 (8.0.12)
 Source Host           : localhost:3306
 Source Schema         : rocway

 Target Server Type    : MySQL
 Target Server Version : 80012 (8.0.12)
 File Encoding         : 65001

 Date: 28/06/2026 21:15:54
*/

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for login_audits
-- ----------------------------
DROP TABLE IF EXISTS `login_audits`;
CREATE TABLE `login_audits`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '用户名',
  `event_type` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '事件类型',
  `failed_count` bigint(20) NOT NULL DEFAULT 0 COMMENT '失败次数',
  `ip` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT 'IP',
  `created_at` bigint(20) NULL DEFAULT NULL COMMENT '创建时间',
  `expires_at` bigint(20) NULL DEFAULT NULL COMMENT '过期时间',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_login_audits_username`(`username` ASC) USING BTREE,
  INDEX `idx_login_audits_event_type`(`event_type` ASC) USING BTREE,
  INDEX `idx_login_audits_created_at`(`created_at` ASC) USING BTREE,
  INDEX `idx_login_audits_expires_at`(`expires_at` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 264 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Records of login_audits
-- ----------------------------
INSERT INTO `login_audits` VALUES (257, 'admin', 'lock_short', 5, '', 1782535197, 1782536097);
INSERT INTO `login_audits` VALUES (263, 'admin', 'lock_long', 10, '', 1782535250, 1782538080);

-- ----------------------------
-- Table structure for login_logs
-- ----------------------------
DROP TABLE IF EXISTS `login_logs`;
CREATE TABLE `login_logs`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '用户名',
  `user_id` bigint(20) UNSIGNED NOT NULL DEFAULT 0 COMMENT '用户ID',
  `status` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '登录状态',
  `reason` varchar(191) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '失败原因',
  `ip` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT 'IP地址',
  `user_agent` varchar(191) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '用户代理',
  `created_at` bigint(20) NULL DEFAULT NULL COMMENT '发生时间',
  PRIMARY KEY (`id`) USING BTREE,
  INDEX `idx_auth_login_logs_username`(`username` ASC) USING BTREE,
  INDEX `idx_auth_login_logs_user_id`(`user_id` ASC) USING BTREE,
  INDEX `idx_auth_login_logs_status`(`status` ASC) USING BTREE,
  INDEX `idx_login_logs_username`(`username` ASC) USING BTREE,
  INDEX `idx_login_logs_user_id`(`user_id` ASC) USING BTREE,
  INDEX `idx_login_logs_status`(`status` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 340 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Records of login_logs
-- ----------------------------
INSERT INTO `login_logs` VALUES (278, 'admin', 0, 'failure', 'user_not_found', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535187);
INSERT INTO `login_logs` VALUES (279, 'admin', 0, 'failure', 'user_not_found', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535191);
INSERT INTO `login_logs` VALUES (280, 'admin', 0, 'failure', 'user_not_found', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535194);
INSERT INTO `login_logs` VALUES (281, 'admin', 0, 'failure', 'user_not_found', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535196);
INSERT INTO `login_logs` VALUES (282, 'admin', 0, 'failure', 'user_not_found', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535197);
INSERT INTO `login_logs` VALUES (283, 'admin', 0, 'locked_attempt', 'account_locked_short', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535199);
INSERT INTO `login_logs` VALUES (284, 'admin', 0, 'locked_attempt', 'account_locked_short', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535245);
INSERT INTO `login_logs` VALUES (285, 'admin', 0, 'locked_attempt', 'account_locked_short', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535247);
INSERT INTO `login_logs` VALUES (286, 'admin', 0, 'locked_attempt', 'account_locked_short', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535249);
INSERT INTO `login_logs` VALUES (287, 'admin', 0, 'locked_attempt', 'account_locked_short', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535250);
INSERT INTO `login_logs` VALUES (288, 'admin', 0, 'locked_attempt', 'account_locked_long', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782535252);
INSERT INTO `login_logs` VALUES (289, 'admin', 0, 'locked_attempt', 'account_locked_long', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782537767);
INSERT INTO `login_logs` VALUES (290, 'admin', 0, 'locked_attempt', 'account_locked_long', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782539149);
INSERT INTO `login_logs` VALUES (291, 'admin', 37, 'success', '', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782539627);
INSERT INTO `login_logs` VALUES (292, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543263);
INSERT INTO `login_logs` VALUES (293, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543269);
INSERT INTO `login_logs` VALUES (294, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543278);
INSERT INTO `login_logs` VALUES (295, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Microsoft Windows 10.0.26200; zh-CN) PowerShell/7.5.5', 1782543287);
INSERT INTO `login_logs` VALUES (296, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543298);
INSERT INTO `login_logs` VALUES (297, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543308);
INSERT INTO `login_logs` VALUES (298, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543326);
INSERT INTO `login_logs` VALUES (299, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543411);
INSERT INTO `login_logs` VALUES (300, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543421);
INSERT INTO `login_logs` VALUES (301, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543485);
INSERT INTO `login_logs` VALUES (302, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782543512);
INSERT INTO `login_logs` VALUES (303, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782548095);
INSERT INTO `login_logs` VALUES (304, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782548104);
INSERT INTO `login_logs` VALUES (305, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782548243);
INSERT INTO `login_logs` VALUES (306, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782548269);
INSERT INTO `login_logs` VALUES (307, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782548746);
INSERT INTO `login_logs` VALUES (308, 'admin', 37, 'success', '', '127.0.0.1', 'curl/8.20.0', 1782549082);
INSERT INTO `login_logs` VALUES (309, 'admin', 37, 'success', '', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782549688);
INSERT INTO `login_logs` VALUES (310, 'admin', 37, 'success', '', '127.0.0.1', 'Apifox/1.0.0 (https://apifox.com)', 1782572249);
INSERT INTO `login_logs` VALUES (311, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782572799);
INSERT INTO `login_logs` VALUES (312, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782573780);
INSERT INTO `login_logs` VALUES (313, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782573833);
INSERT INTO `login_logs` VALUES (314, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782573874);
INSERT INTO `login_logs` VALUES (315, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782574024);
INSERT INTO `login_logs` VALUES (316, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782613163);
INSERT INTO `login_logs` VALUES (317, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782613322);
INSERT INTO `login_logs` VALUES (318, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782613362);
INSERT INTO `login_logs` VALUES (319, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782613431);
INSERT INTO `login_logs` VALUES (320, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782613648);
INSERT INTO `login_logs` VALUES (321, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782613696);
INSERT INTO `login_logs` VALUES (322, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782613892);
INSERT INTO `login_logs` VALUES (323, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782613896);
INSERT INTO `login_logs` VALUES (324, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782613946);
INSERT INTO `login_logs` VALUES (325, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782614023);
INSERT INTO `login_logs` VALUES (326, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782614058);
INSERT INTO `login_logs` VALUES (327, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782614141);
INSERT INTO `login_logs` VALUES (328, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782614159);
INSERT INTO `login_logs` VALUES (329, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782614185);
INSERT INTO `login_logs` VALUES (330, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782614205);
INSERT INTO `login_logs` VALUES (331, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782614447);
INSERT INTO `login_logs` VALUES (332, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782614659);
INSERT INTO `login_logs` VALUES (333, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782614834);
INSERT INTO `login_logs` VALUES (334, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782615829);
INSERT INTO `login_logs` VALUES (335, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782627491);
INSERT INTO `login_logs` VALUES (336, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782627739);
INSERT INTO `login_logs` VALUES (337, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782627781);
INSERT INTO `login_logs` VALUES (338, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782627907);
INSERT INTO `login_logs` VALUES (339, 'admin', 37, 'success', '', '127.0.0.1', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0', 1782628021);

-- ----------------------------
-- Table structure for users
-- ----------------------------
DROP TABLE IF EXISTS `users`;
CREATE TABLE `users`  (
  `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '用户名',
  `email` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '邮箱',
  `nickname` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '昵称',
  `password` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '密码',
  `avatar` text CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NULL COMMENT '头像',
  `created_at` bigint(20) NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` bigint(20) NULL DEFAULT NULL COMMENT '更新时间',
  `deleted_at` datetime(3) NULL DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_users_username`(`username` ASC) USING BTREE,
  INDEX `idx_users_email`(`email` ASC) USING BTREE,
  INDEX `idx_users_deleted_at`(`deleted_at` ASC) USING BTREE
) ENGINE = InnoDB AUTO_INCREMENT = 38 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = Dynamic;

-- ----------------------------
-- Records of users
-- ----------------------------
INSERT INTO `users` VALUES (37, 'admin', 'admin@example.com', 'rocway', '$2a$10$DSvibtlkflMq/uUYke6NSu9Ofw4s622deiu5dPjaloNZOiFujf4xe', NULL, 1782538628, 1782538628, NULL);

SET FOREIGN_KEY_CHECKS = 1;
