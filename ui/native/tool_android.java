package com.coyace.rtc.tool;

import android.Manifest;
import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.view.View;
import android.webkit.MimeTypeMap;

import androidx.core.content.FileProvider;

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
        Optional<String> type = Optional.ofNullable(ctx.getContentResolver().getType(uri));
        if (type.isEmpty()) {
            uri = FileProvider.getUriForFile(
                    ctx,
                    ctx.getPackageName() + ".fileprovider",
                    new File(path)
            );
        }
        intent.setDataAndType(uri, type.orElseGet(() -> getMimeType(path)));
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION
                | Intent.FLAG_ACTIVITY_NEW_TASK
                | Intent.FLAG_ACTIVITY_MULTIPLE_TASK // 允许多个实例
        );

        // 启动 Activity
        if (type.isEmpty()) {
            Intent chooser = Intent.createChooser(intent, "选择应用打开文件");
            chooser.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
            ctx.startActivity(chooser);
            return;
        }
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