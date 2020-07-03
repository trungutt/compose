/*
   Copyright 2020 Docker, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package convert

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/pkg/errors"

	"github.com/compose-spec/compose-go/types"

	"github.com/docker/api/errdefs"
)

// GetRunVolumes return volume configurations for a project and a single service
// this is meant to be used as a compose project of a single service
func GetRunVolumes(volumes []string) (map[string]types.VolumeConfig, []types.ServiceVolumeConfig, error) {
	var serviceConfigVolumes []types.ServiceVolumeConfig
	projectVolumes := make(map[string]types.VolumeConfig, len(volumes))
	for i, v := range volumes {
		var vi volumeInput
		err := vi.parse(fmt.Sprintf("volume-%d", i), v)
		if err != nil {
			return nil, nil, err
		}
		projectVolumes[vi.name] = types.VolumeConfig{
			Name:   vi.name,
			Driver: azureFileDriverName,
			DriverOpts: map[string]string{
				volumeDriveroptsAccountNameKey: vi.username,
				volumeDriveroptsAccountKeyKey:  vi.key,
				volumeDriveroptsShareNameKey:   vi.share,
			},
		}
		sv := types.ServiceVolumeConfig{
			Type:   azureFileDriverName,
			Source: vi.name,
			Target: vi.target,
		}
		serviceConfigVolumes = append(serviceConfigVolumes, sv)
	}

	return projectVolumes, serviceConfigVolumes, nil
}

type volumeInput struct {
	name     string
	username string
	key      string
	share    string
	target   string
}

func escapeKeySlashes(rawURL string) (string, error) {
	urlSplit := strings.Split(rawURL, "@")
	if len(urlSplit) < 1 {
		return "", fmt.Errorf("invalid URL format: %s", rawURL)
	}
	userPasswd := strings.ReplaceAll(urlSplit[0], "/", "_")

	atIndex := strings.Index(rawURL, "@")
	if atIndex < 0 {
		return "", fmt.Errorf("no share specified in: %s", rawURL)
	}

	scaped := userPasswd + rawURL[atIndex:]

	return scaped, nil
}

func unescapeKey(key string) string {
	return strings.ReplaceAll(key, "_", "/")
}

// Removes the second ':' that separates the source from target
func volumeURL(pathURL string) (*url.URL, error) {
	scapedURL, err := escapeKeySlashes(pathURL)
	if err != nil {
		return nil, err
	}
	pathURL = "//" + scapedURL

	count := strings.Count(pathURL, ":")
	if count > 2 {
		return nil, fmt.Errorf("invalid path URL: %s", pathURL)
	}
	if count == 2 {
		tokens := strings.Split(pathURL, ":")
		pathURL = fmt.Sprintf("%s:%s%s", tokens[0], tokens[1], tokens[2])
	}
	return url.Parse(pathURL)
}

func (v *volumeInput) parse(name string, s string) error {
	volumeURL, err := volumeURL(s)
	if err != nil {
		return errors.Wrapf(errdefs.ErrParsingFailed, "unable to parse volume specification: %s", err.Error())
	}
	v.username = volumeURL.User.Username()
	if v.username == "" {
		return errors.Wrapf(errdefs.ErrParsingFailed, "volume specification %q does not include a storage username", v)
	}
	key, ok := volumeURL.User.Password()
	if !ok || key == "" {
		return errors.Wrapf(errdefs.ErrParsingFailed, "volume specification %q does not include a storage key", v)
	}
	v.key = unescapeKey(key)
	v.share = volumeURL.Host
	if v.share == "" {
		return errors.Wrapf(errdefs.ErrParsingFailed, "volume specification %q does not include a storage file share", v)
	}
	v.name = name
	v.target = volumeURL.Path
	if v.target == "" {
		// Do not use filepath.Join, on Windows it will replace / by \
		v.target = "/run/volumes/" + v.share
	}
	return nil
}
