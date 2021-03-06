package grafana

/*
   Copyright 2016 Alexander I.Grafov <grafov@gmail.com>
   Copyright 2016-2019 The Grafana SDK authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

	   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.

   ॐ तारे तुत्तारे तुरे स्व
*/

type User struct {
	ID             uint   `json:"id"`
	Login          string `json:"login"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Theme          string `json:"theme"`
	OrgID          uint   `json:"orgId"`
	Password       string `json:"password"`
	IsGrafanaAdmin bool   `json:"isGrafanaAdmin"`
}

type UserRole struct {
	LoginOrEmail string `json:"loginOrEmail"`
	Role         string `json:"role"`
}

type UserPermissions struct {
	IsGrafanaAdmin bool `json:"isGrafanaAdmin"`
}

type UserPassword struct {
	Password string `json:"password"`
}

type PageUsers struct {
	TotalCount int    `json:"totalCount"`
	Users      []User `json:"users"`
	Page       int    `json:"page"`
	PerPage    int    `json:"perPage"`
}
