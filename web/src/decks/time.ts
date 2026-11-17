export function relative_time(value: string) {
  const minutes = Math.max(
    0,
    Math.floor((Date.now() - new Date(value).getTime()) / 60000),
  );
  return minutes < 1
    ? "刚刚"
    : minutes < 60
      ? `${minutes} 分钟前`
      : minutes < 1440
        ? `${Math.floor(minutes / 60)} 小时前`
        : minutes < 2880
          ? "昨天"
          : minutes < 14 * 1440
            ? `${Math.floor(minutes / 1440)} 天前`
            : `${Math.floor(minutes / (7 * 1440))} 周前`;
}
