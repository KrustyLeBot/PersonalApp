package com.krustylebot.personaldashboard;

import android.os.Bundle;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(AppBridgePlugin.class);
        super.onCreate(savedInstanceState);
    }
}
