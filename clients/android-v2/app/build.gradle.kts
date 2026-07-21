plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.hilt)
    alias(libs.plugins.ksp)
}

fun environmentValue(name: String): String? =
    providers.environmentVariable(name).orNull?.takeIf { it.isNotBlank() }

val releaseStoreFile = environmentValue("ANDROID_SIGNING_STORE_FILE")
val releaseStorePassword = environmentValue("ANDROID_SIGNING_STORE_PASSWORD")
val releaseKeyAlias = environmentValue("ANDROID_SIGNING_KEY_ALIAS")
val releaseKeyPassword = environmentValue("ANDROID_SIGNING_KEY_PASSWORD")
val releaseSigningValues = listOf(
    releaseStoreFile,
    releaseStorePassword,
    releaseKeyAlias,
    releaseKeyPassword,
)
val hasAnyReleaseSigningValue = releaseSigningValues.any { it != null }
val hasReleaseSigning = releaseSigningValues.all { it != null }

check(!hasAnyReleaseSigningValue || hasReleaseSigning) {
    "Android V2 release signing is partially configured. Set ANDROID_SIGNING_STORE_FILE, " +
        "ANDROID_SIGNING_STORE_PASSWORD, ANDROID_SIGNING_KEY_ALIAS and ANDROID_SIGNING_KEY_PASSWORD together."
}

val versionNameInput = environmentValue("ANDROID_VERSION_NAME")
val versionCodeInput = environmentValue("ANDROID_VERSION_CODE")
check((versionNameInput == null) == (versionCodeInput == null)) {
    "ANDROID_VERSION_NAME and ANDROID_VERSION_CODE must be configured together. " +
        "Use scripts/android-v2-version.sh to derive the version code."
}
val resolvedVersionCode = versionCodeInput?.toIntOrNull()
    ?: if (versionCodeInput == null) 100101 else error("ANDROID_VERSION_CODE must be a positive integer.")
check(resolvedVersionCode > 0) { "ANDROID_VERSION_CODE must be a positive integer." }
val resolvedVersionName = versionNameInput ?: "0.1.0-alpha.1"

android {
    namespace = "com.nowen.video.v2"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.nowen.video.v2"
        minSdk = 26
        targetSdk = 35
        versionCode = resolvedVersionCode
        versionName = resolvedVersionName
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    signingConfigs {
        if (hasReleaseSigning) {
            create("release") {
                storeFile = file(releaseStoreFile!!)
                storePassword = releaseStorePassword
                keyAlias = releaseKeyAlias
                keyPassword = releaseKeyPassword
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            if (hasReleaseSigning) {
                signingConfig = signingConfigs.getByName("release")
            }
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
        debug {
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }

    packaging {
        resources.excludes += "/META-INF/{AL2.0,LGPL2.1}"
    }
}

dependencies {
    implementation(project(":feature:main"))
    implementation(project(":core:data"))
    implementation(project(":core:model"))
    implementation(project(":core:designsystem"))
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.material3)
    implementation(libs.hilt.android)
    ksp(libs.hilt.compiler)
    implementation(libs.media3.exoplayer)
    implementation(libs.media3.session)
    implementation(libs.androidx.work.runtime)
    implementation(libs.coil.compose)
    implementation(libs.okhttp)
    debugImplementation(libs.androidx.compose.ui.tooling)
}
