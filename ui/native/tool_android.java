package com.coyace.rtc.tool;

import android.Manifest;
import android.app.Activity;
import android.content.pm.PackageManager;
import android.view.View;

import java.io.File;
import java.util.Optional;

public class tool_android {
    public static void askPermission(View view) {
        Activity activity = (Activity) view.getContext();
        if (activity.checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) {
            activity.requestPermissions(new String[]{Manifest.permission.RECORD_AUDIO}, 128);
        }
    }

    public static String getExternalDir(View view) {
        Activity activity = (Activity) view.getContext();
        return Optional.ofNullable(activity.getExternalFilesDir(null))
                .map(File::getAbsolutePath)
                .orElse("");
    }
}