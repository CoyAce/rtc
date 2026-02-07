package com.coyace.rtc.tool;

import android.Manifest;
import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.view.View;
import android.webkit.MimeTypeMap;

import java.io.File;
import java.util.Optional;

public class tool_android {
    public static void askPermission(View view) {
        Activity activity = (Activity) view.getContext();
        if (activity.checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) {
            activity.requestPermissions(new String[]{Manifest.permission.RECORD_AUDIO}, 128);
        }
    }

    public static String getExternalDir(Context ctx) {
        return Optional.ofNullable(ctx.getExternalFilesDir(null))
                .map(File::getAbsolutePath)
                .orElse("");
    }

    public static void browseFile(Context ctx, String path) {
        // 获取文件的 Uri
        Uri uri = Uri.parse(path);
        // 创建 Intent
        Intent intent = new Intent(Intent.ACTION_VIEW);
        // 设置 MIME 类型
        String mime = Optional.ofNullable(ctx.getContentResolver().getType(uri)).orElse(getMimeType(path));
        intent.setDataAndType(uri, mime);
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_ACTIVITY_NEW_TASK);

        // 启动 Activity
        ctx.startActivity(intent);
    }

    private static String getMimeType(String fileName) {
        String extension = getFileExtension(fileName).toLowerCase();
        return MimeTypeMap.getSingleton().getMimeTypeFromExtension(extension);
    }

    private static String getFileExtension(String fileName) {
        int dotIndex = fileName.lastIndexOf('.');
        if (dotIndex > 0 && dotIndex < fileName.length() - 1) {
            return fileName.substring(dotIndex + 1);
        }
        return "";
    }
}