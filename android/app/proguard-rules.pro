# =============================================
# Nowen Video ProGuard 规则 — Phase 4 完善版
# =============================================

# ==================== Kotlin Serialization ====================
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.AnnotationsKt
-keepclassmembers class kotlinx.serialization.json.** {
    *** Companion;
}
-keepclasseswithmembers class kotlinx.serialization.json.** {
    kotlinx.serialization.KSerializer serializer(...);
}
-keep,includedescriptorclasses class com.nowen.video.data.model.**$$serializer { *; }
-keepclassmembers class com.nowen.video.data.model.** {
    *** Companion;
}
-keepclasseswithmembers class com.nowen.video.data.model.** {
    kotlinx.serialization.KSerializer serializer(...);
}

# WebSocket 事件模型也需要保留序列化
-keep,includedescriptorclasses class com.nowen.video.data.remote.WSEvent { *; }
-keep,includedescriptorclasses class com.nowen.video.data.remote.ScanProgressData { *; }
-keep,includedescriptorclasses class com.nowen.video.data.remote.ScrapeProgressData { *; }
-keep,includedescriptorclasses class com.nowen.video.data.remote.TranscodeProgressData { *; }
-keepclassmembers class com.nowen.video.data.remote.WSEvent { *; }
-keepclassmembers class com.nowen.video.data.remote.ScanProgressData { *; }
-keepclassmembers class com.nowen.video.data.remote.ScrapeProgressData { *; }
-keepclassmembers class com.nowen.video.data.remote.TranscodeProgressData { *; }

# ServerProfile 序列化
-keep,includedescriptorclasses class com.nowen.video.data.local.ServerProfile { *; }
-keepclassmembers class com.nowen.video.data.local.ServerProfile { *; }

# ==================== Retrofit ====================
-keepattributes Signature, InnerClasses, EnclosingMethod
-keepattributes RuntimeVisibleAnnotations, RuntimeVisibleParameterAnnotations
-keepclassmembers,allowshrinking,allowobfuscation interface * {
    @retrofit2.http.* <methods>;
}
-dontwarn org.codehaus.mojo.animal_sniffer.IgnoreJRERequirement
-dontwarn javax.annotation.**
-dontwarn kotlin.Unit
-dontwarn retrofit2.KotlinExtensions
-dontwarn retrofit2.KotlinExtensions$*

# 保留 API 接口
-keep interface com.nowen.video.data.remote.NowenApiService { *; }

# ==================== OkHttp ====================
-dontwarn okhttp3.**
-dontwarn okio.**
-keep class okhttp3.** { *; }
-keep interface okhttp3.** { *; }

# ==================== ExoPlayer / Media3 ====================
-keep class androidx.media3.** { *; }
-dontwarn androidx.media3.**

# ==================== Room ====================
-keep class * extends androidx.room.RoomDatabase
-keep @androidx.room.Entity class *
-dontwarn androidx.room.paging.**

# ==================== DataStore ====================
-keepclassmembers class * extends com.google.protobuf.GeneratedMessageLite {
    <fields>;
}

# ==================== Hilt / Dagger ====================
-dontwarn dagger.hilt.internal.aggregatedroot.codegen.**
-keep class dagger.hilt.** { *; }
-keep class javax.inject.** { *; }
-keep class * extends dagger.hilt.android.internal.managers.ViewComponentManager$FragmentContextWrapper { *; }

# ==================== Compose ====================
-dontwarn androidx.compose.**

# ==================== Coil ====================
-dontwarn coil.**

# ==================== Kotlin Coroutines ====================
-keepnames class kotlinx.coroutines.internal.MainDispatcherFactory {}
-keepnames class kotlinx.coroutines.CoroutineExceptionHandler {}
-keepclassmembers class kotlinx.coroutines.** {
    volatile <fields>;
}

# ==================== 通用规则 ====================
-keepattributes SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile
