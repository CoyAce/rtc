package com.coyace.rtc.recorder;

import android.Manifest;
import android.app.Activity;
import android.content.pm.PackageManager;
import android.view.View;

public class recorder_android {
    public static void askPermission(View view) {
        Activity activity = (Activity) view.getContext();
        if (activity.checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) {
            activity.requestPermissions(new String[]{Manifest.permission.RECORD_AUDIO}, 128);
        }
    }
}