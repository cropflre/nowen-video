package com.nowen.video.ui.screen.mobile

import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.TokenManager
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 移动端设置页 UI 状态
 */
data class MobileSettingsUiState(
    val username: String = "",
    val userRole: String = "",
    val serverUrl: String = "",
    val appVersion: String = "",
    val isLoggedIn: Boolean = false,
)

/**
 * 移动端设置页 ViewModel
 */
@HiltViewModel
class MobileSettingsViewModel @Inject constructor(
    private val tokenManager: TokenManager,
    @ApplicationContext private val context: Context,
) : ViewModel() {

    private val _uiState = MutableStateFlow(MobileSettingsUiState())
    val uiState = _uiState.asStateFlow()

    init {
        loadSettings()
    }

    fun loadSettings() {
        viewModelScope.launch {
            val username = tokenManager.getUsername() ?: ""
            val userRole = tokenManager.getUserRole() ?: ""
            val serverUrl = tokenManager.getServerUrl() ?: ""
            val appVersion = getAppVersion()

            _uiState.value = MobileSettingsUiState(
                username = username,
                userRole = userRole,
                serverUrl = serverUrl,
                appVersion = appVersion,
                isLoggedIn = username.isNotBlank(),
            )
        }
    }

    private fun getAppVersion(): String {
        return try {
            val packageInfo = context.packageManager.getPackageInfo(context.packageName, 0)
            packageInfo.versionName ?: "unknown"
        } catch (e: Exception) {
            "unknown"
        }
    }
}
