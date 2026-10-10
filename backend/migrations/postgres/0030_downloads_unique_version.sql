-- 下载中心:同 (software_name, platform, version) 的重复行会让 CheckVersion
-- 的 sort.Slice(非稳定排序)在相等版本键间随机取行,latest 及其 force_update/
-- changelog 随请求翻转。先清重(保留 id 最大 = 最新一行),再加唯一约束根治。

DELETE FROM als_downloads
WHERE id NOT IN (
    SELECT MAX(id) FROM als_downloads GROUP BY software_name, platform, version
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_als_downloads_software_platform_version
    ON als_downloads (software_name, platform, version);
